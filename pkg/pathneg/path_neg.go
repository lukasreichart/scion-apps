// Copyright 2021 ETH Zurich
// Author: Lukas Reichart <lukasre@ethz.ch>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pathneg

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"
)

// ## Implementation ToDos
// todo: need a way that old connections are cleaned out of the state table
// todo: how to protect against state exhaustion attacks (limit by ip)
// todo: need a way to cleanly shutdown the listen method
// todo: error handling -> at least log the errors that occur in the go loops
// todo: callback on server is not implemented
// todo: paths that are logged on the server look like garbage.

// Some general parameters of the protocol that can be changed.
const (
	// the maximum number of paths that can be sent in a single path proposal
	// the protocol takes the top $MaxNumberOfPathsInProposal as determined by the PathEvaluator.
	MaxNumberOfPathsInProposal = 2
	// the amount of times a message should be retried before we assume failure
	MaxNumberOfRetries = 5
	// after how much time the negotiation state should be cleaned up, currently set to one hour
	NegotiationCleanupAfter time.Duration = time.Hour
)

// the different states of the protocol stack, state is kept per negotiation
type NegState uint8
const (
	// the negotiation channel is closed, action is required to change into a listen or proposal sent state.
	NegStateClosed NegState = 0
	// the negotiation channel is ready to receive an offer
	NegStateListen NegState = 1
	// a proposal was sent, now waiting for the response from the other party
	// resend the path proposal if we do not here back
	NegStateProposalSent NegState = 2
	// has responded to a proposal waiting for an ack of the proposal to start using the path
	// resend the path accept if we do not here back.
	NegStatePathAcceptSent NegState = 3
	// the negotiation succeeded, path can now be used.
	NegStateSuccess NegState = 4
	// some error occurred, action needed to put protocol stack back into a listen state.
	NegStateError NegState = 5
)
var stateName = map[NegState]string{
	NegStateClosed:         "closed",
	NegStateListen:         "listen",
	NegStateProposalSent:   "proposal_sent",
	NegStatePathAcceptSent: "accept_sent",
	NegStateSuccess:        "success",
	NegStateError:          "error",
}
func (c NegState) String() string {
	return stateName[c]
}

// Protocol is the main interface of the protocol
type Protocol interface {
	// Listen should start the server part of the protocol waiting for incoming path negotiation requests on fixed port.
	Listen(ctx context.Context, cb func([]snet.Path)) error
	// Negotiate should start the negotiation process with the path negotiation stack running at the addressed in raddr.
	Negotiate(ctx context.Context, raddr *snet.UDPAddr, cb func([]snet.Path)) error
}

// atomicBool implementation taken from the golang http package
type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }
func (b *atomicBool) setFalse()   { atomic.StoreInt32((*int32)(b), 0) }

// pathNegProtocol is the current POC implementation of the path neg protocol.
type pathNegProtocol struct {
	// specifies the address the server is listening on.
	Addr string
	// the PathEvaluator used to evaluate path preferences.
	// at the moment a single PathEvaluator object is used for all negotiations
	evaluator          PathEvaluator
	// the PathSelector used to select the path based on the preferences
	selector PathSelector
	// transport is a helper to send messages
	transport MessageTransport
	// stores the state for active negotiations
	activeNegotiations map[string]*negControlBlock
}

// negControlBlock contains all the relevant state that needs be kept for the negotiation with a single host.
type negControlBlock struct {
	// connection id
	connectionId int32

	// remote addr that we are negotiation with
	remoteAddr snet.UDPAddr

	// the message that was last sent for this negotiation, used for retransmission
	currentMessage atomic.Value // of *Message

	// the state this is negotiation is currently in.
	state NegState

	// retransmit timer
	// todo: add timer here

	// the last time this negotiation session was active, this is used to clean up unused sessions.
	lastAction time.Time
}

// NewPathNeg creates a new instance to manage the path neg protocol through, an application should probably only
// used one of these instances.
func NewPathNeg(port uint16) (Protocol, error) {
	transport, _ := newMessageTransport(port)
	return &pathNegProtocol{
		transport:          transport,
		evaluator:          &ConstantPathEvaluator{},
		activeNegotiations: make(map[string]*negControlBlock, 0),
	}, nil
}

// Listen starts to listen for protocol messages
func (p*pathNegProtocol) Listen(ctx context.Context, cb func([]snet.Path)) error {
	c := make(chan MessageInfo)
	quit := make(chan bool)
	go p.transport.Receive(c, quit)

	for {
		select {
		//case <- p.getDoneChan():
		//	return ErrServerClose

		case msg, ok := <-c:
			if ok {
				if err := p.handleMessage(ctx, msg, cb); err != nil {
					log.Errorf("An error occurred while handling the protocol message: %v\n", err)
				}
			} else {
				fmt.Println("Channel closed!")
			}
		}
	}
}

// Negotiate performs a path negotiation with the host at the passed address
// if the negotiation succeeds the callback is called
func (p*pathNegProtocol) Negotiate(ctx context.Context, raddr *snet.UDPAddr, cb func([]snet.Path)) error {
	// construct the path proposal message
	proposal, err := p.constructPathProposal(raddr)
	if err != nil {
		return err
	}
	log.WithField("PathProposal", proposal).Info("Path proposal constructed\n")

	// make sure that we are ready to receive the  response from the other host.
	log.Info("Setup channel to receive protocol responses")

	// start listening for the response
	c := make(chan MessageInfo)
	quit := make(chan bool)
	go p.transport.Receive(c, quit)

	// create a path negotiation state object to store the state of the negotiation
	negState := newNegState(raddr, NegStateProposalSent)
	negState.currentMessage.Store(proposal)
	p.activeNegotiations[getNegIdentifier(raddr, negState.connectionId)] = negState

	log.Info("Send path proposal message")

	if err := p.sendMessageHelper(negState, proposal, raddr); err != nil {
		return err
	}

	log.Info("Waiting for response")

	// block until we get a return message
	msg := <- c
	log.Info("Received response message")
	if err := p.handleMessage(ctx, msg, cb); err != nil {
		close(c)
		quit<-true
		close(quit)
		return err
	}
	close(c)
	quit<-true
	close(quit)

	return nil
}

// handleMessage calls the correct message handler based on the message type.
func (p*pathNegProtocol) handleMessage(ctx context.Context, msg MessageInfo, cb func([]snet.Path)) error {
	switch msg.Header.MessageType {
	// we have received a path negotiation proposal from another party
	case MessageTypePathProposal:
		return p.handlePathProposalMessage(msg, cb)
	case MessageTypePathAccept:
		return p.handlePathAcceptMessage(msg, cb)
	case MessageTypeError:
		return p.handleErrorMessage(msg)
	default:
		return fmt.Errorf("Message type %d was not recognized.\n", msg.Header.MessageType)
	}
}

// handlePathProposalMessage handles an incoming PathProposal message
func (p*pathNegProtocol) handlePathProposalMessage(msg MessageInfo, cb func([]snet.Path)) error {
	proposal := msg.Payload.(PathProposal)
	log.Info("Received a PathProposal message")

	negIdentifier := getNegIdentifierForMessage(&msg)
	negState, ok := p.activeNegotiations[negIdentifier];

	// if we receive a path neg proposal we better be in the listen state for that ip
	if ok {
		// check that we are in the listen state
		if negState.state != NegStateListen {
			return serrors.New("Illegal state")
		}
	} else {
		// create a new state object for that ip
		negState = &negControlBlock{
			// use the connection id received in the message
			connectionId:    msg.Header.ConnectionId,
			remoteAddr:      msg.From,
			currentMessage:  atomic.Value{},
			state:           NegStateListen,
			lastAction:      time.Time{},
		}
		p.activeNegotiations[negIdentifier] = negState
	}

	// check that we have received at least as many Paths as need to be negotiated
	if len(proposal.Paths) < proposal.NumberOfPaths {
		return serrors.New("Not enough path were sent.")
	}

	// todo: here we would have the fancy logic to pick the path
	log.Info("Calculating local path preferences.")

	log.Info("Constructing PathAccept message")
	accept := PathAccept{
		Paths: []snet.Path{proposal.Paths[0].Copy()},
	}

	log.WithField("Accepted Path", fmt.Sprintf("%s", proposal.Paths[0])).Info("Server has accepted path:")

	// update the negotiation state
	negState.state = NegStateProposalSent
	// store the message for possible retransmit
	negState.currentMessage.Store(accept)

	log.Info("Sending PathAccept message back")
	if err := p.sendMessageHelper(negState, accept, &msg.From); err != nil {
		return err
	}

	//log.WithField("Path", fmt.Sprintf("%s", accept.Paths[0])).Info("Negotiation success: ")

	cb(accept.Paths)

	return nil
}

// handlePathAccpetMessage handles an incoming PathAccept message
func (p*pathNegProtocol) handlePathAcceptMessage(msg MessageInfo, cb func([]snet.Path)) error {
	log.Info("Received a PathAccept message")
	accept := msg.Payload.(PathAccept)

	negIdentifier := getNegIdentifierForMessage(&msg)
	negState, ok := p.activeNegotiations[negIdentifier];
	if !ok {
		return serrors.New("No state found can not respond to accept")
	}

	if negState.state != NegStateProposalSent {
		return serrors.New("State missmatch, needs to be in state proposal sent to work here.")
	}
	//
	// todo: check that the Paths that were accepted, were also part of the Paths

	//Paths were accepted so we can now communicate this to the user of the library?
	negState.state = NegStateSuccess

	log.WithField("Path accept", accept).Info("Path accept message")

	log.WithField("Path", fmt.Sprintf("%s", accept.Paths[0])).Info("Negotiation success: ")

	// inform the host about the new paths
	cb(accept.Paths)

	return nil
}

// handleErrorMessage handles an incoming Error message.
func (p*pathNegProtocol) handleErrorMessage(msg MessageInfo) error {
	log.Error("Received a path negotiation errMsg from the other side.")

	negIdentifier := getNegIdentifierForMessage(&msg)
	negState, ok := p.activeNegotiations[negIdentifier];

	// todo: is there a better way to handle this errMsg?
	errMsg := msg.Payload.(PathError)

	if !ok {
		// we do not have state for this what should we do?
		// for now just ignore it
	} else {
		negState.state = NegStateError
	}

	fmt.Printf("Error : %v\n", errMsg)

	return nil
}

// constructPathProposal constructs a path proposal message that can be sent as an
// offering the path negotiation protocol
func (p*pathNegProtocol) constructPathProposal(raddr *snet.UDPAddr) (*PathProposal, error){
	log.Infof("Querying Paths to address %s", raddr.String())

	// query the (public) Paths to the destination address.
	publicPaths, err := appnet.QueryPaths(raddr.IA)
	if err != nil {
		return nil, err
	}

	log.Info("Calculating local preferences of Paths to %s", raddr.String())
	// get the Weights for the Paths
	weights := p.evaluator.WeightsForPaths(publicPaths)

	log.WithField(
		"Paths", publicPaths).WithField(
		"weights", weights).Info("Calculated the weights for all the Paths:\n")


	// get the number of paths that we want to send
	numberOfPaths := len(publicPaths)
	if numberOfPaths > MaxNumberOfPathsInProposal {
		numberOfPaths = MaxNumberOfPathsInProposal
	}

	log.Infof("Constructing the path proposal message with a total of %d Paths", numberOfPaths)

	// sort the Paths
	sort.Slice(publicPaths, func(i1, i2 int) bool {
		return weights[i1] > weights[i2]
	})
	sort.Slice(weights, func(i1, i2 int) bool {
		return weights[i1] > weights[i2]
	})

	// construct the PathProposal message, using the top n Paths.
	return &PathProposal{
		Paths:         publicPaths[0:numberOfPaths],
		Weights:       weights[0:numberOfPaths],
		NumberOfPaths: numberOfPaths,
	}, nil
}

func (p*pathNegProtocol) sendMessageHelper(negState *negControlBlock, payload Message,
	to *snet.UDPAddr) error {
	msgInfo := MessageInfo{
		Header:  MessageHeaders{
			ProtocolVersion: 1,
			MessageNumber:   0,
			AckNumber:       0,
			MessageType:     payload.Type(),
			ConnectionId:    negState.connectionId,
		},
		Payload: payload,
	}

	return p.transport.Send(msgInfo, to)
}

func newNegState(raddr *snet.UDPAddr, state NegState) *negControlBlock {
	return &negControlBlock{
		connectionId:	 rand.Int31(),
		remoteAddr:      *raddr,
		currentMessage:  atomic.Value{},
		state:           state,
		lastAction:      time.Time{},
	}
}

func getNegIdentifier(raddr *snet.UDPAddr, connectionId int32) string {
	return raddr.IA.String() + raddr.Host.IP.String() + fmt.Sprintf("%d", connectionId)
}

func getNegIdentifierForMessage(msg *MessageInfo) string {
	return getNegIdentifier(&msg.From, msg.Header.ConnectionId)
}





