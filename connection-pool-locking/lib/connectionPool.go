package lib

import (
	"io"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"
	pb "github.com/chainforce/free-peer/connection-pool-locking/chaincode_proto"
	"google.golang.org/grpc"
	"context"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

const (
	address = "127.0.0.1:50051"
)

type ConnectionPoolWrapper struct {
	Size        int
	LiveConnMap map[int]*ConnectionWrapper
	DeadConnMap map[int]*ConnectionWrapper
	ConnNum     int
	Mutex sync.Mutex
}

type ConnectionWrapper struct {
	Id         int
	ClientConn interface{}
	InitFn     InitClientConnFunction
	CloseFn    CloseClientConnFunction
	TimeToLive int
	Requests   int
	Dead       bool
	Mutex sync.Mutex
}

func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	p.LiveConnMap = make(map[int]*ConnectionWrapper)
	p.DeadConnMap = make(map[int]*ConnectionWrapper)
	for i := 0; i < size; i++ {
		c := p.newConnection(ttL, initFn, closeFn)
		go p.runConnection(c)
		time.Sleep(time.Second)
	}
	p.Size = size
	return nil
}

func (p *ConnectionPoolWrapper) GetConnection(k int) *ConnectionWrapper {
	return p.LiveConnMap[k]
}

func (p *ConnectionPoolWrapper) newConnection(ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *ConnectionWrapper {
	c := &ConnectionWrapper{}
	c.InitFn = initFn
	c.CloseFn = closeFn
	c.TimeToLive = ttL
	return c
}

func (p *ConnectionPoolWrapper) runConnection(c *ConnectionWrapper) {
	err := p.initConnection(c)
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}

	// Wait for Time to Live
	time.Sleep(time.Duration(c.TimeToLive) * time.Second)

	// Reset connection
	killed, err := p.killConnection(c)
	if err != nil {
		log.Fatalf("error killing connection: %v", err)
	}
	for killed != true {
		log.Printf("error killing connection, connection in use, retrying in 1 second")
		time.Sleep(1 * time.Second)
		killed, err = p.killConnection(c)
		if err != nil {
			log.Fatalf("error killing connection: %v", err)
		}
	}

	newC := &ConnectionWrapper{
		InitFn: c.InitFn,
		CloseFn: c.CloseFn,
		TimeToLive: c.TimeToLive,
	}
	
	p.runConnection(newC)
}

func (p *ConnectionPoolWrapper) initConnection(c *ConnectionWrapper) error {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	p.ConnNum++
	c.Id = p.ConnNum
	log.Printf("Starting connection: %v", strconv.Itoa(c.Id))
	clientConn, err := c.InitFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	p.LiveConnMap[c.Id] = c
	log.Printf("Started connection: %v", strconv.Itoa(c.Id))
	return nil
}

func (p *ConnectionPoolWrapper) killConnection(c *ConnectionWrapper) (bool, error) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	log.Printf("Killing connection: %v", strconv.Itoa(c.Id))
	if c.Requests == 0 {
		delete(p.LiveConnMap, c.Id)
		err := c.CloseFn(c.ClientConn)
		if err != nil {
			return false, err
		}
		c.Dead = true
		p.DeadConnMap[c.Id] = c
		log.Printf("Killed connection: %v", strconv.Itoa(c.Id))
		return true, nil
	}
	return false, nil
}

func (p *ConnectionPoolWrapper) Send(ccReq *pb.ChaincodeRequest) error {
	c := p.GetConnection(p.ConnNum)
	
	clientConn := c.ClientConn.(*grpc.ClientConn)
	client := pb.NewChaincodeClient(clientConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.ChaincodeChat(ctx)
	if err != nil {
		log.Fatalf("chaincode chat failed on connection %v: %v", c.Id, err)
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
			log.Printf(" | %v recieved on connection: %v", in.Message, c.Id)
			c.Requests--
			log.Printf("c.Request reduced to: %v", c.Requests)
		}
	}()

	if err := stream.Send(ccReq); err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}
	c.Requests++
	log.Printf("c.Request increased to: %v", c.Requests)

	if err := stream.CloseSend(); err != nil {
		log.Fatalf("error sending close on stream: %v", err)
	}
	<-waitChan
	log.Printf("Live connections:")
	var live []int
	for k := range p.LiveConnMap {
		live = append(live, k)
	}
	sort.Ints(live)
	log.Printf("%v", live)

	var dead []int
	log.Printf("Dead connections:")
	for k := range p.DeadConnMap {
		dead = append(dead, k)
	}
	sort.Ints(dead)
	log.Printf("%v", dead)

	return nil
}

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

func InitGrpcPool(size, ttL int) (*ConnectionPoolWrapper, error) {
	var p = &ConnectionPoolWrapper{}
	err := p.InitPool(size, ttL, initGrpcConnection, closeGrpcConnection)
	if err != nil {
		return nil, err
	}
	return p, nil
}
