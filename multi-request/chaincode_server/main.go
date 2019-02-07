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

	pb "github.com/chainforce/free-peer/multi-request/chaincode_proto"
	"google.golang.org/grpc"
)

const (
	port = ":50051"
)


// server is used to implement helloworld.GreeterServer.
type chaincode struct{
	chaincodeName string
}

type CCHandler struct {
	sync.Mutex
	ongoingTxs map[int32]chan*pb.ChaincodeRequest
	stream pb.Chaincode_ChaincodeChatClient
}

func (c *chaincode) ChaincodeChat(stream pb.Chaincode_ChaincodeChatServer) error {
	h := NewCCHandler(stream)
	h.Start()
}

func (h *CCHandler) Start() {
	h.ongoingTxs = map[int32]chan*pb.ChaincodeRequest{}

	for {
		reqFromPeer, err := h.stream.Recv()
		if err != nil {
			log.Fatalf("error receiving from stream: %v", err)
		}
		if reqFromPeer.IsTX {
			if _, ok := h.ongoingTxs[reqFromPeer.TxID]; ok {
				log.Fatalf("error duplication tx: %v", reqFromPeer)
			}

			ch := make(chan*pb.ChaincodeResponse)
			h.ongoingTxs[reqFromPeer.TxID] = make(chan*pb.ChaincodeRequest)

			go func(tx *pb.ChaincodeRequest, ch chan*pb.ChaincodeResponse) {
				for i := 0; i < 3; i++ {
					reqFromCC := &pb.ChaincodeRequest{Input: fmt.Sprintf("CC PutState: %v", i), IsTX: false, TxID: reqFromPeer.TxID}
					err := h.stream.Send(reqFromCC)
					if err != nil {
						log.Fatalf("error sending on stream: %v", err)
					}

					resp := <-ch
					if resp.TxID != tx.TxID {
						log.Fatalf("error request txID mismatch")
					}
				}
				doneFromCC := &pb.ChaincodeRequest{Input: "Done", IsTX: false, TxID: reqFromPeer.TxID}
				err := h.stream.Send(doneFromCC)
				if err != nil {
					log.Fatalf("error sending on stream: %v", err)
				}
				h.TXDone(reqFromPeer.TxID)
			}(reqFromPeer, ch)
		} else {
			ch := h.ongoingTxs[reqFromPeer.TxID]
			ch <- reqFromPeer
		}
	}
	conn, err := grpc.Dial()
	client := pb.NewChaincodeClient()
	stream, err := client.ChaincodeChat()
	stream.Send()
	stream.Recv()
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	serv := newChaincode()
	pb.RegisterChaincodeServer(s, serv)
	log.Println("CC: " + serv.chaincodeName + " started.")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func newChaincode() *chaincode {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := &chaincode{chaincodeName: strconv.Itoa(r.Int())}
	return s
}


