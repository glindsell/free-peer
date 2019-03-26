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
	"sync"
	"time"

pb "github.com/chainforce/free-peer/app/chaincode_proto"
"google.golang.org/grpc"
)

const (
	port = ":50051"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	chaincodeName string
}

type chaincodeHandler struct {
	sync.Mutex
	ongoingTxs map[int32]chan *pb.ChaincodeMessage
	stream pb.Chaincode_ChaincodeChatServer
}

func newChaincodeHandler(stream pb.Chaincode_ChaincodeChatServer) *chaincodeHandler {
	ongTx := make(map[int32]chan *pb.ChaincodeMessage)
	handler := &chaincodeHandler{
		ongoingTxs: ongTx,
		stream: stream,
	}
	return handler
}

func (s *server) ChaincodeChat(stream pb.Chaincode_ChaincodeChatServer) error {
	h := newChaincodeHandler(stream)
	err := h.Start()
	if err != nil {
		return err
	}
	return nil
}

func (h *chaincodeHandler) Start() error {
	for {
		reqFromPeer, err := h.stream.Recv()
		if err == io.EOF {
			log.Printf("EOF received")
			return nil
		}
		if err != nil {
			return err
		}
		log.Printf(" | RECV <<< Req: %v\n", reqFromPeer)
		if reqFromPeer.IsTX {
			if _, ok := h.ongoingTxs[reqFromPeer.TxID]; ok {
				log.Fatal("error: duplication TX received by chaincode!")
			}
			h.ongoingTxs[reqFromPeer.TxID] = make(chan *pb.ChaincodeMessage)
			go func(requestIn *pb.ChaincodeMessage) {
				for i := 0; i < 3; i++ {
					message := fmt.Sprintf("CHAINCODE REQUEST %v for tx: %v", i, requestIn.TxID)
					reqFromCC := &pb.ChaincodeMessage{TxID: requestIn.TxID, Message: message}
					err := h.stream.Send(reqFromCC)
					if err != nil {
						log.Fatalf(fmt.Sprintf("error: %v", err))
					}
					log.Printf(" | SEND >>> Req: %v\n", reqFromCC)
					resp := <-h.ongoingTxs[reqFromPeer.TxID]
					if resp.TxID != requestIn.TxID {
						log.Fatal("error: request ID mismatch in chaincode")
					}
				}

				respDone := &pb.ChaincodeMessage{TxID: reqFromPeer.TxID, Message: "CHAINCODE DONE"}
				err = h.stream.Send(respDone)
				if err != nil {
					log.Fatalf(fmt.Sprintf("error: %v", err))
				}
			}(reqFromPeer)
		} else {
			ch := h.ongoingTxs[reqFromPeer.TxID]
			ch <- reqFromPeer
			log.Printf("Req is ongoing tx: %v", reqFromPeer.Message)
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
