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
	"context"
	"fmt"
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
	address = "127.0.0.1:50052"
	port = ":50051"
)

// server is used to implement helloworld.GreeterServer.
type chatServer struct {
	chatServerName string
}

type CCHandler struct {
	sync.Mutex
	serverStream pb.ChatService_ChatServer
	clientStream pb.ChatService_ChatClient
	//chatServiceServer pb.ChatServiceServer
	chatServiceClient pb.ChatServiceClient
}

func (c *chatServer) Chat(servStream pb.ChatService_ChatServer) error {
	h := NewCCHandler(servStream)
	h.Start()
	return nil
}

func NewCCHandler (servStream pb.ChatService_ChatServer) *CCHandler {
	h := &CCHandler{}
	h.serverStream = servStream
	return h
}

func (h *CCHandler) SendReq(msg *pb.ChatRequest) error {
	//h.Lock()
	//defer h.Unlock()
	err := h.clientStream.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (h *CCHandler) RecvResp() (*pb.ChatResponse, error) {
	//h.Lock()
	//defer h.Unlock()
	resp, err := h.clientStream.Recv()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *CCHandler) RecvReq() (*pb.ChatRequest, error) {
	//h.Lock()
	//defer h.Unlock()
	in, err := h.serverStream.Recv()
	if err != nil {
		return nil, err
	}
	return in, nil
}

func (h *CCHandler) SendResp(msg *pb.ChatResponse) error {
	//h.Lock()
	//defer h.Unlock()
	err := h.serverStream.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (h *CCHandler) Start() {
	grpcCon, err := initGrpcConnection()
	if err != nil {
		log.Fatalf("error starting grpc connection: %v", err)
	}
	clientConnection := grpcCon.(*grpc.ClientConn)
	for {
		msgFromPeer, err := h.RecvReq()
		if err != nil {
			log.Fatalf("error receiving from stream: %v", err)
		}
		log.Printf("Received: %v", msgFromPeer)

		go func(req *pb.ChatRequest) {
			h.chatServiceClient = pb.NewChatServiceClient(clientConnection)
			ctx := context.Background()
			h.clientStream, err = h.chatServiceClient.Chat(ctx) // This was nil
			if err != nil {
				log.Fatalf("error creating client stream: %v", err)
			}
			log.Println("Chat started")

			for i := 0; i < 3; i++ {
				log.Printf("i: %v", i)
				reqFromCC := &pb.ChatRequest{Input: fmt.Sprintf("CC PutState: %v", i), IsTX: false, TxID: msgFromPeer.TxID}
				err := h.SendReq(reqFromCC)
				if err != nil {
					log.Fatalf("error sending on stream: %v", err)
				}
				log.Printf("1")
				log.Printf("Request sent: %v", reqFromCC)
				peerResp, err := h.RecvResp()
				log.Printf("Response received: %v", peerResp)

				if peerResp.TxID != req.TxID {
					log.Fatalf("error request txID mismatch")
				}
			}

			doneFromCC := &pb.ChatRequest{Input: "CHAINCODE DONE", TxID: msgFromPeer.TxID}
			err = h.SendReq(doneFromCC)
			if err != nil {
				log.Fatalf("error sending on stream: %v", err)
			}
			log.Printf("Done sent")
		}(msgFromPeer)
	}
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	serv := newChatServer()
	pb.RegisterChatServiceServer(s, serv)
	log.Println("CC: " + serv.chatServerName + " started.")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func newChatServer() *chatServer {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := &chatServer{chatServerName: strconv.Itoa(r.Int())}
	return s
}

func initGrpcConnection() (interface{}, error) {
	// Create connection
	grpcConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return grpcConn, nil
}
