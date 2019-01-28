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
	"io"
	"log"
	"os"
	"time"

	pb "github.com/chainforce/free-peer/uni-stream/chaincode_proto"
	"google.golang.org/grpc"
)

const (
	address        = "127.0.0.1:50051"
	defaultRequest = "CC-Request"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewChaincodeClient(conn)

	// Contact the server and print out its response.
	request := defaultRequest
	if len(os.Args) > 1 {
		request = defaultRequest + " " + os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := c.MakeRequest(ctx, &pb.ChaincodeRequest{Input: request})
	if err != nil {
		log.Fatalf("could not send request: %v", err)
	}
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}
		log.Printf(reply.Message)
	}
}
