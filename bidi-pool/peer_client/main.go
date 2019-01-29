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

package main

import (
	"context"
	pb "github.com/chainforce/free-peer/bidi-pool/chaincode_proto"
	"google.golang.org/grpc"
	"io"
	"log"
	"os"
	"strconv"
)

const (
	address        = "127.0.0.1:50051"
	defaultRequest = "Peer name: "
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewChaincodeClient(conn)

	// Contact the server and print out its response.
	request := defaultRequest
	if len(os.Args) > 1 {
		request = defaultRequest + os.Args[1]
	}

	ctx := context.Background()
	//defer cancel()
	stream, err := client.ChaincodeChat(ctx)
	if err != nil {
		log.Fatalf("%v.ChaincodeChat(_) = _, %v", client, err)
	}

	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive message: %v", err)
			}
			log.Printf(" | %v", in.Message)
		}
	}()

	numReqs, err:= strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("Failed parsing command line args: %v", err)
	}

	for i := 0; i < numReqs ; i++ {
		if err := stream.Send(&pb.ChaincodeRequest{Input: request + " | Request: " + strconv.Itoa(i)}); err != nil {
			log.Fatalf("Failed to send request: %v", err)
		}
	}
	stream.CloseSend()
	<-waitc
}
