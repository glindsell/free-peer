package main

import (
	"fmt"
	pb "github.com/chainforce/free-peer/connection-pool-locking/chaincode_proto"
	"github.com/chainforce/free-peer/connection-pool-locking/lib"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/trace"
	"time"
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
	p, err := lib.InitGrpcPool(3, 3000)
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
	for i = 0; i < 100; i++ {
		SendTx(p, &pb.ChaincodeRequest{Input: "Request message", IsTX: true, TxID: i})
		r := rand.Intn(1000)
		time.Sleep(time.Duration(r) * time.Millisecond)
	}
	// prevent main from exiting immediately
	//var input string
	//fmt.Scanln(&input)
}

func SendTx(p *lib.ConnectionPoolWrapper, txReq *pb.ChaincodeRequest) {
	if !txReq.IsTX {
		log.Fatalf("error: SendTx on a peer connection handler should be a TX")
	}

	h, err := p.GetConnectionHandler()
	if err != nil {
		log.Fatalf("%v", err)
	}

	/*if _, ok := h.OngoingTxs[txReq.TxID]; ok {
		log.Fatalf("error: Duplicate tx")
	}*/
	waitc := make(chan struct{})
	go func(tx *pb.ChaincodeRequest) {
		for {
			resp, err := h.RecvResp()
			if err != nil {
				log.Fatalf("%v", err)
			}

			if resp.TxID != tx.TxID {
				log.Fatalf("error: bad req, txid mismatch")
			}
			if resp.Message == "done" {
				err = h.CloseSend()
				close(waitc)
				if err != nil {
					log.Fatalf("%v", err)
				}
				return
			}
			reqMessage := fmt.Sprintf("peer req, in response to chaincode response: %v", resp.Message)
			req := &pb.ChaincodeRequest{Input: reqMessage, IsTX: false, TxID: resp.TxID}

			err = h.SendReq(req)
			if err != nil {
				log.Fatalf("%v", err)
			}
		}
	}(txReq)

	err = h.SendReq(txReq)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//ch := make(chan *pb.ChaincodeResponse)

	//h.OngoingTxs[txReq.TxID] = make(chan *pb.ChaincodeRequest)

	<- waitc
}
