package main

import (
	"fmt"
	pb "github.com/chainforce/free-peer/app/chaincode_proto"
	"github.com/chainforce/free-peer/app/lib"
	"log"
	"os"
	"runtime"
	"runtime/trace"
)

func main() {
	runtime.GOMAXPROCS(10)
	f, err := os.Create("trace.out")
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer f.Close()

	err = trace.Start(f)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer trace.Stop()

	// Create a pool of connections using the initPool function
	p, err := lib.InitGrpcPool(4, 500)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// create go routine on each connection
	// call chaincode chat on each go routine
	// receive stream in for loop, same as chaincode server
	// has same map of txid to go channel req

	// send tx from peer
	// creates new go routine
	// registers go routine with map of txid to go channel
	var i int32
	for i = 0; i < 10000; i++ {
		input := fmt.Sprintf("PEER REQUEST - TX: %v START - message", i)
		go SendTx(p, &pb.ChaincodeMessage{Message: input, IsTX: true, TxID: i})
		//r := rand.Intn(1000)
		//time.Sleep(time.Duration(r) * time.Millisecond)
	}
	log.Printf("*** FINISHED ***")
	// prevent main from exiting immediately
	var input string
	fmt.Scanln(&input)
}

func SendTx(p *lib.ConnectionPool, txReq *pb.ChaincodeMessage) {
	if !txReq.IsTX {
		log.Fatalf("error: SendTx on a peer connection handler should be a TX")
	}

	h, err := p.GetConnectionHandler()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("Got connection: %v", h.Connection.Id)

	err = h.SendTx(txReq)
	if err != nil {
		log.Fatalf("%v", err)
	}
}
