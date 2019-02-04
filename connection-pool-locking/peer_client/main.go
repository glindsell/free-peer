package main

import (
	"math/rand"
	"os"
	"runtime"
	"runtime/trace"

	pb "github.com/chainforce/free-peer/connection-pool-locking/chaincode_proto"
	"github.com/chainforce/free-peer/connection-pool-locking/lib"
	"log"
	"strconv"
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
	p, err := lib.InitGrpcPool(3, 3)
	if err != nil {
		log.Fatalf("%v", err)
	}

	//time.Sleep(time.Second)

		for i := 0; i < 100; i++ {
			err = p.Send(&pb.ChaincodeRequest{Input: "Request: " + strconv.Itoa(i)})
			if err != nil {
				log.Fatalf("error sending chaincode request: %v", err)
			}
			r := rand.Intn(1000)
			time.Sleep(time.Duration(r) * time.Millisecond)
		}
	// prevent main from exiting immediately
	/*var input string
	fmt.Scanln(&input)*/
}
