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
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
)

// MessageTransport implements  the transport for the path negotiation protocol messages
type MessageTransport interface {
	Send(message MessageInfo, raddr *snet.UDPAddr) error
	Receive(channel chan MessageInfo, quit chan bool) error
}

type messageTransport struct {
	// the port that should be used to listen for connections
	port uint16
}

func newMessageTransport(port uint16) (MessageTransport, error) {
	return &messageTransport{
		port,
	}, nil
}

// Send sends the message over the passed connection to the passed address.
func (p *messageTransport) Send(message MessageInfo, raddr *snet.UDPAddr) error {
	// todo: check if this really sets the port correctly
	raddr.Host.Port = int(p.port)

	conn, err := appnet.DialAddr(raddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// build the container
	container := &MessageContainer{
		Bytes:       nil,
		MessageInfo: message,
	}

	// serialize the message
	if err := container.Serialize(); err != nil {
		return err
	}

	// send the message
	n, err := conn.Write(container.Bytes)
	if err != nil {
		return fmt.Errorf("")
	}
	if n != len(container.Bytes) {
		return fmt.Errorf("Not all data could be sent.\n")
	}

	return nil
}

// todo: need an error channel here to communicate errors back
// Receive handles the receival and decoding of messages over the passed connection.
func (p *messageTransport) Receive(channel chan MessageInfo, quit chan bool) error {
	conn, err := appnet.ListenPort(p.port)
	if err != nil {
		return err
	}

	buffer := make([]byte, 32 * 1024)
	for {
		select {
			case <- quit:
				return conn.Close()
			default:
				n, from, err := conn.ReadFrom(buffer)

				fromUdp := from.(*snet.UDPAddr)

				if err != nil {
					return nil
				}

				// deserialize the data to a message
				packet := MessageContainer{
					MessageInfo: MessageInfo{
						From:    *fromUdp,
					},
					Bytes:       buffer[0:n],
				}
				// headers and payload will be set by the decode method
				if err := packet.Decode(); err != nil {
					return err
				}

				// send the message over the channel
				channel <- packet.MessageInfo
		}
	}
}







