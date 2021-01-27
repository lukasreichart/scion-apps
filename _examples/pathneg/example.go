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

package main

// example.go is a simple example on how to use the path negotiation protocol.


import (
	"context"
	"flag"
	"fmt"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/pathneg"
	"github.com/scionproto/scion/go/lib/snet"
	"os"
	log "github.com/sirupsen/logrus"
)

func main() {
	// get the type of host to run server or client
	server := flag.Bool("server",false,"Mark execution to be a server")
	addr := flag.String("address", "", "Specify the address to perform path negotiation with")

	flag.Parse()

	if *server {
		if err := runServer(); err != nil {
			log.Errorf("Failed to start app server with error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *addr == "" {
		log.Errorf("Please specify the address of the host to perform path negotiation with.")
	}

	if err := runClient(*addr); err != nil {
		log.Errorf("Failed to run client with error: %v\n", err)
		os.Exit(1)
	}
}

func printPaths(paths *[]snet.Path) {
	//fmt.Printf("Available Paths to %v\n", raddr.IA)
	for i, path := range *paths {
		fmt.Printf("[%2d] %s\n", i, fmt.Sprintf("%s", path))
	}
}

func newPathHandler(paths []snet.Path) {
	printPaths(&paths)
}

func runServer() error {
	log.Info("Starting the application server.")

	log.Info("Starting up the path negotiation server.")

	// start up the path negotiation server
	pathNeg, err := pathneg.NewPathNeg(2222)
	if err != nil {
		return fmt.Errorf("Failed to create path neg protocol instance %v\n", err)
	}

	if err := pathNeg.Listen(context.Background(), newPathHandler); err != nil {
		return fmt.Errorf("Failed to start path neg server %v\n")
	}
	log.Info("Path Negotiation Server is now ready for connections.")

	return nil
}

func runClient(addr string) error {
	log.Info("Starting the path negotiation client")
	pathNeg, err := pathneg.NewPathNeg(2222)
	if err != nil {
		return fmt.Errorf("Failed to create path neg protocol instance %v\n", err)
	}

	//raddr, err := appnet.ResolveUDPAddr("17-ffaa:1:e8b,[127.0.0.1]:2222")
	raddr, err := appnet.ResolveUDPAddr(addr)
	if err != nil {
		return err
	}

	log.Info("Negotiating Paths with remote server.")
	paths, err := pathNeg.Negotiate(context.Background(), raddr)
	if err != nil {
		return fmt.Errorf("Failed to negotiate path %v\n", err)
	}
	printPaths(&paths)

	return nil
}
