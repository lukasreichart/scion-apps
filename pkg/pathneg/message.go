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
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/path"
)

// message.go defines the messages used by the path negotiation protocol.

// MessageType represents the different types of messages that we send in the protocol
type MessageType uint8
const(
	MessageTypePathProposal MessageType = 1
	MessageTypePathAccept   MessageType = 2
	MessageTypeAck          MessageType = 3
	MessageTypeError        MessageType = 4
)
var messageTypeName = map[MessageType]string {
	MessageTypePathProposal: "path_proposal",
	MessageTypePathAccept: "path_accept",
	MessageTypeAck: "ack",
	MessageTypeError: "error",
}
func (t MessageType) String() string {
	return messageTypeName[t]
}

// ErrorCode represents the different errors that can occur in the protocol
type ErrorCode uint8
const (
	// the party denies the offering send by the initial party
	ErrorCodePathDeny ErrorCode = 1
)
var errorCodeName = map[ErrorCode]string {
	ErrorCodePathDeny: "error_deny",
}
func (e ErrorCode) String() string {
	return errorCodeName[e]
}

// the headers of the path negotiation message
type MessageHeaders struct {
	// the version of the path negotiation protocol that is used, currently set to 1
	ProtocolVersion uint8
	// the number of the message
	MessageNumber uint8
	// the ack number of the message
	AckNumber uint8
	// the type of message
	MessageType MessageType
	// ConnectionId
	ConnectionId int32
}
// the size of the header in bytes
func (h MessageHeaders) SizeInBytes() int {
	// size is currently 4 since we have 4 x uint8 (1 byte) + int32 (4 bytes) = 8 bytes.
	return 8
}

// Message is the interface for any message in the path negotiation protocol.
type Message interface {
	// get the type of the message
	Type() MessageType
}

// PathMsgBasic combines all the fields that are contained in all messages.
type PathMsgBasic struct {
	//// the address the message was received from (since we are currently using UDP as transport we use a UDPAddr here).
	//From snet.UDPAddr
}

// PathProposal represents the proposal to use certain paths from one host to the other.
type PathProposal struct {
	PathMsgBasic
	// the paths that are offered by the sending host
	Paths   []snet.Path
	// the relative weights of the paths, same order as the path array.
	Weights []PathWeight
	// the number of Paths that should be negotiated
	NumberOfPaths int
}
func (p PathProposal) Type() MessageType {
	return MessageTypePathProposal
}

// PathAccept represents one party accepting the path proposal of the other party.
type PathAccept struct {
	PathMsgBasic
	// the paths that were accepted.
	// todo: check if we really need to send the full paths here or whether we can just send the indices and a reference to the proposal
	// invariant len(Paths) = PathProposal.NumberOfPaths
	Paths []snet.Path
}
func (p PathAccept) Type() MessageType {
	return MessageTypePathAccept
}

// PathError represents an error message.
type PathError struct {
	PathMsgBasic

	// the identifier of the error that occurred.
	code ErrorCode
}
func (p PathError) Type() MessageType {
	return MessageTypeError
}

// MessageInfo contains the Message itself as well as Headers and additional info needed for processing it.
type MessageInfo struct {
	// the address from which this message was received
	From snet.UDPAddr
	// the headers of the message
	Header MessageHeaders
	// payload of the mesage
	Payload Message
}

// MessageContainer is used as a Container for a path negotiation message and its byte representation.
type MessageContainer struct {
	// the serialized bytes to send
	snet.Bytes
	// representation of the actual message
	MessageInfo
}

// Serialize the MessageInfo in to the raw buffer of the message
func (m *MessageContainer) Serialize() error {
	// todo: check if we can do this better.
	var payloadBytes bytes.Buffer
	gob.RegisterName("path.Path", path.Path{})
	enc := gob.NewEncoder(&payloadBytes)
	err := enc.Encode(m.Payload)
	if err != nil {
		return serrors.New("Failed to serialize message: %v\n", err)
	}

	// make a buffer size payload + headers
	m.Bytes = make([]byte, len(payloadBytes.Bytes()) + m.Header.SizeInBytes())
	// copy over the payload
	n := copy(m.Bytes[m.Header.SizeInBytes():],payloadBytes.Bytes())
	if n != len(payloadBytes.Bytes()) {
		return serrors.New("Failed to copy")
	}
	// set the message header
	m.Bytes[0] = m.Header.ProtocolVersion
	m.Bytes[1] = m.Header.MessageNumber
	m.Bytes[2] = m.Header.AckNumber
	m.Bytes[3] = byte(m.Payload.Type())
	binary.BigEndian.PutUint32(m.Bytes[4:8], uint32(m.Header.ConnectionId))

	return nil
}

// Decode decodes the bytes in the message into the MessageInfo field.
func (m *MessageContainer) Decode() error {
	// decode the header
	m.Header = MessageHeaders{
		ProtocolVersion: m.Bytes[0],
		MessageNumber:   m.Bytes[1],
		AckNumber:       m.Bytes[2],
		MessageType:     MessageType(m.Bytes[3]),
		ConnectionId: 	 int32(binary.BigEndian.Uint32(m.Bytes[4:8])),
	}
	if m.Header.MessageType > 5 {
		// unsupported message type
		return serrors.New("Path negotiation unsupported message type")
	}

	// set the from field

	// decode the message
	buffer := bytes.NewBuffer(m.Bytes[8:])
	gob.RegisterName("path.Path", path.Path{})
	dec := gob.NewDecoder(buffer)

	switch m.Header.MessageType {
	case MessageTypePathProposal:
		var payload PathProposal
		err := dec.Decode(&payload)
		if err != nil {
			return fmt.Errorf("Failed to decode message: %v\n", err)
		}
		m.Payload = payload
	case MessageTypePathAccept:
		var payload PathAccept
		err := dec.Decode(&payload)
		if err != nil {
			return fmt.Errorf("Failed to decode message: %v\n", err)
		}
		m.Payload = payload
	case MessageTypeError:
		var payload PathError
		err := dec.Decode(&payload)
		if err != nil {
			return fmt.Errorf("Failed to decode message: %v\n", err)
		}
		m.Payload = payload
	default:
		return serrors.New("Unsupported message type")
	}
	return nil
}
