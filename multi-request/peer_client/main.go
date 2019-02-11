package main

import (
	"fmt"
	pb "github.com/chainforce/free-peer/multi-request/chaincode_proto"
	"github.com/chainforce/free-peer/multi-request/lib"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/trace"
	"time"
)

type chatServer struct {
	chatServerName string
}

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
	p, err := lib.InitGrpcPool(3, 5000)
	if err != nil {
		log.Fatalf("%v", err)
	}

	go lib.Serve()

	var i int32
	for {
		log.Println("Go")
		input := fmt.Sprintf("PEER REQUEST - TX: %v START - message", i)
		SendTx(p, &pb.ChatRequest{Input: input, IsTX: true, TxID: i})
		r := rand.Intn(1000)
		time.Sleep(time.Duration(r) * time.Millisecond)
		i++
	}

	// prevent main from exiting immediately
	//var input string
	//fmt.Scanln(&input)
}

func SendTx(p *lib.ConnectionPoolWrapper, txReq *pb.ChatRequest) {
	if !txReq.IsTX {
		log.Fatalf("error: SendTx on a peer connection handler should be a TX")
	}

	h, err := p.GetConnectionHandler()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("Got connection: %v", h.ConnectionWrapper.Id)

	err = h.SendReq(txReq)
	if err != nil {
		log.Fatalf("%v", err)
	}
}
