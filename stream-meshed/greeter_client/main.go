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
	"google.golang.org/grpc"
	pb "github.com/chainforce/free-peer/stream-meshed/helloworld"
	"log"
	"os"
	"time"
)

var (
	//port 		= os.Getenv("SERVER_PORT")
	address     = os.Getenv("SERVERSVC_HOST")
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx := context.Background()

	for i := 0; i < 10000; i++ {

		r, err := c.SayHello(ctx)
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}
		err = r.Send(&pb.HelloMessage{Message: "Message from Peer"})
		if err != nil {
			log.Fatal(err)
		}
		in, err := r.Recv()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Peer received: %s", in.Message)

		time.Sleep(1000 * time.Millisecond)
	}
}
