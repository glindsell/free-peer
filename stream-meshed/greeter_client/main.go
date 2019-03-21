package main

import (
	"context"
	"fmt"
	pb "github.com/chainforce/free-peer/stream-meshed/helloworld"
	"google.golang.org/grpc"
	"log"
	"time"
)

var (
	address = "ingress.local:32145"
	podName = "client-1"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewGreeterClient(conn)
	ctx := context.Background()

	for {
		for i := 0; i < 10; i++ {
			go func() {
				r, err := c.SayHello(ctx)
				if err != nil {
					log.Fatalf("could not greet: %v", err)
				}
				err = r.Send(&pb.HelloMessage{Message: fmt.Sprintf("Message from Peer: %v", podName)})
				if err != nil {
					log.Fatal(err)
				}
				in, err := r.Recv()
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("Peer received: %s", in.Message)
			}()
		}
		time.Sleep(time.Second)
	}
	var input string
	fmt.Scanln(&input)
}
