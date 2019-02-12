package main

import (
	"fmt"
	pb "github.com/chainforce/free-peer/single-msg-type/chaincode_proto"
	"github.com/chainforce/free-peer/single-msg-type/lib"
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
	for j := 0; j < 1000; j++ {
		input := fmt.Sprintf("PEER REQUEST - TX: %v START - message", i)
		SendTx(p, &pb.ChaincodeMessage{Message: input, IsTX: true, TxID: i})
		//r := rand.Intn(1000)
		//time.Sleep(time.Duration(r) * time.Millisecond)
		i++
	}
	// prevent main from exiting immediately
	var input string
	fmt.Scanln(&input)
}

func SendTx(p *lib.ConnectionPoolWrapper, txReq *pb.ChaincodeMessage) {
	if !txReq.IsTX {
		log.Fatalf("error: SendTx on a peer connection handler should be a TX")
	}

	h, err := p.GetConnectionHandler()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("Got connection: %v", h.ConnectionWrapper.Id)

	waitc := make(chan struct{})
	err = h.SendReq(txReq)
	if err != nil {
		log.Fatalf("%v", err)
	}
	go func(tx *pb.ChaincodeMessage) {
		for {
			resp, err := h.RecvResp()
			if err != nil {
				log.Fatalf("%v", err)
			}
			if resp.TxID != tx.TxID {
				log.Fatalf("error: bad req, txid mismatch")
			}
			if resp.Message == "CHAINCODE DONE" {
				p.ReleaseConnection(h.ConnectionWrapper.Id)
				err = h.CloseSend()
				if err != nil {
					log.Fatalf("%v", err)
				}
				close(waitc)
				return
			}
			reqMessage := fmt.Sprintf("PEER RESPONSE OK to: %v", resp.Message)
			req := &pb.ChaincodeMessage{Message: reqMessage, IsTX: false, TxID: resp.TxID}

			err = h.SendReq(req)
			if err != nil {
				log.Fatalf("%v", err)
			}
		}
	}(txReq)
	<- waitc
}
