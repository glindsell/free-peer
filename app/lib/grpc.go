package lib

import (
	"google.golang.org/grpc"
	"log"
)

func initGrpcConnection() (interface{}, error) {
	// Create connection
	grpcConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return grpcConn, nil
}

func closeGrpcConnection(grpcConn interface{}) error {
	// Create connection
	conn := grpcConn.(*grpc.ClientConn)
	err := conn.Close()
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return nil
}

func InitGrpcPool(size, ttL int) (*ConnectionPool, error) {
	var p = &ConnectionPool{}
	err := p.InitPool(size, ttL, initGrpcConnection, closeGrpcConnection)
	if err != nil {
		return nil, err
	}
	return p, nil
}
