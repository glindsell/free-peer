/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

//go:generate protoc -I ./ ./*.proto --go_out=plugins=grpc:./

package main

import (
"fmt"
"io"
"log"
"math/rand"
"net"
"strconv"
"time"

pb "github.com/chainforce/free-peer/connection-pool-locking/chaincode_proto"
"google.golang.org/grpc"
)

const (
	port = ":50051"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	chaincodeName string
}

func (s *server) ChaincodeChat(stream pb.Chaincode_ChaincodeChatServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Printf("EOF received")
			return nil
		}
		if err != nil {
			return err
		}
		log.Printf(" | RECV <<< Req: %v\n", req)
		if req.IsTX {
			go func(request *pb.ChaincodeRequest) {
				for i := 0; i < 3; i++ {
					respMessage := fmt.Sprintf("CHAINCODE REQUEST - PUT STATE %v - for tx: %v", i, req.TxID)
					resp := &pb.ChaincodeResponse{Message: respMessage, TxID: req.TxID}
					err := stream.Send(resp)
					if err != nil {
						log.Fatalf(fmt.Sprintf("error: %v", err))
					}
					log.Printf(" | SEND >>> Req: %v\n", resp)
				}

				respDone := &pb.ChaincodeResponse{Message: "CHAINCODE DONE", TxID: req.TxID}
				err = stream.Send(respDone)
				if err != nil {
					log.Fatalf(fmt.Sprintf("error: %v", err))
				}
			}(req)
		} else {
			log.Printf("Req is ongoing tx: %v", req.Input)
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	serv := newServer()
	pb.RegisterChaincodeServer(s, serv)
	log.Println("CC: " + serv.chaincodeName + " started.")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func newServer() *server {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := &server{chaincodeName: strconv.Itoa(r.Int())}
	return s
}
