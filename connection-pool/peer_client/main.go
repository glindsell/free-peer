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

 //TODO: Fix errors

package main

import (
	"context"
	pb "github.com/chainforce/free-peer/bidi-pool/chaincode_proto"
	"github.com/chainforce/free-peer/connection-pool/lib"
	"google.golang.org/grpc"
	"io"
	"log"
	"strconv"
	"time"
)

const (
	address        = "127.0.0.1:50051"
)

var connPool = &lib.ConnectionPoolWrapper{}

type connFunction func(conn *lib.ConnectionWrapper) error

/**
 This function creates a connection to the database. It shouldn't have to know anything
 about the pool, It will be called N times where N is the size of the requested pool.
*/
func initConnection() (interface{}, error) {
	// Create connection
	grpcConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return grpcConn, nil
}

func closeConnection(grpcConn interface{}) error {
	// Create connection
	conn := grpcConn.(*grpc.ClientConn)
	err := conn.Close()
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return nil
}

func UseConnection(fn connFunction) error {
	var conn = &lib.ConnectionWrapper{}

	// Grab connection from the pool
	conn = connPool.GetConnection()
	log.Printf("Got connection: %v", conn.Id)
	// When this function exits, release the connection back to the pool
	defer connPool.ReleaseConnection(conn)

	if conn == nil {
		log.Printf("No open connections available! Retrying...")
		err := UseConnection(fn)
		if err != nil {
			log.Fatalf("error getting connection: %v", err)
		}
	}


	// Do work
	err := fn(conn)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	// Create a pool of connections using the initPool function
	err := connPool.InitPool(3, 1, initConnection, closeConnection)
	if err != nil {
		log.Fatalf("%v", err)
	}
	// Wait for connections to be set-up
	time.Sleep(1 * time.Second)
	for {
		err = UseConnection(sendTx)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}
}

func sendTx(conn *lib.ConnectionWrapper) error {
	clientConn := conn.ClientConn.(*grpc.ClientConn)
	client := pb.NewChaincodeClient(clientConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.ChaincodeChat(ctx)
	if err != nil {
		log.Fatalf("chaincode chat failed on connection %v: %v", conn.Id, err)
	}

	waitChan := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitChan)
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive message: %v", err)
			}
			log.Printf(" | %v recieved on connection: %v", in.Message, conn.Id)
		}
	}()

	for i := 0; i < 3 ; i++ {
		if err := stream.Send(&pb.ChaincodeRequest{Input: "Request: " + strconv.Itoa(i)}); err != nil {
			log.Fatalf("Failed to send request: %v", err)
		}
	}

	if err := stream.CloseSend(); err != nil {
		log.Fatalf("error sending close on stream: %v", err)
	}
	log.Println("Stream closed")
	<-waitChan

	return nil
}
