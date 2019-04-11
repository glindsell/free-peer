package main

import (
	"context"
	//"crypto/tls"
	//"crypto/x509"
	"fmt"
	pb "github.com/chainforce/free-peer/stream-meshed/helloworld"
	"google.golang.org/grpc"
	//"google.golang.org/grpc/credentials"
	//"io/ioutil"
	"log"
	"time"
)

var (
	addr    = "ingress.local:31822"
	podName = "client-1"
	crt     = "/Users/george/ssl/certstrap/out/ingress.local.crt"
	//key     = "/Users/george/ssl/certstrap/out/127.0.0.1.key"
	//ca      = "/Users/george/ssl/certstrap/out/127.0.0.1.crt"
)

func main() {

	/*// Load the client certificates from disk
	certificate, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		log.Fatalf("could not load client key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(ca)
	if err != nil {
		log.Fatalf("could not read ca certificate: %s", err)
	}

	// Append the certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Fatalf("failed to append ca certs")
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "ingress.local", // NOTE: this is required!
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})

	// Create a connection with the TLS credentials
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("could not dial %s: %s", addr, err)
	}*/

	/*// Read cert file
	FrontendCert, err := ioutil.ReadFile(crt)
	if err != nil {
		log.Fatalf("error reading cert: %s", err.Error())
	}

	// Create CertPool
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(FrontendCert); !ok {
		log.Fatal("error appending cert")
	}

	// Create credentials
	credsClient := credentials.NewClientTLSFromCert(roots, "")

	// Dial with specific transport
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(credsClient))
	if err != nil {
		log.Fatalf("fail to dial: %s", err.Error())
	}*/

	// Dial with insecure
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("fail to dial: %s", err.Error())
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
