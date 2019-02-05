package lib

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/chainforce/free-peer/connection-pool-locking/chaincode_proto"
	"google.golang.org/grpc"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"
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
	Mutex       sync.Mutex
}

type ConnectionWrapper struct {
	Id         int
	ClientConn interface{}
	InitFn     InitClientConnFunction
	CloseFn    CloseClientConnFunction
	TimeToLive int
	Requests   int
	Dead       bool
	Handlers   []*ConnectionHandler
	Mutex      sync.Mutex
}

type ConnectionHandler struct {
	ConnectionWrapper *ConnectionWrapper
	chatClient        *pb.Chaincode_ChaincodeChatClient // allows gRPC stream
	chatServer        *pb.Chaincode_ChaincodeChatServer // allows gRPC stream
	cancelContext     context.CancelFunc
	OngoingTxs        map[int32]chan *pb.ChaincodeRequest
	Mutex             sync.Mutex
}

func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	p.LiveConnMap = make(map[int]*ConnectionWrapper)
	p.DeadConnMap = make(map[int]*ConnectionWrapper)
	for i := 0; i < size; i++ {
		c := p.newConnection(ttL, initFn, closeFn)
		go p.runConnection(c)
		time.Sleep(1000 * time.Millisecond)
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
	p.PrintConnectionMaps()

	// Wait for Time to Live
	time.Sleep(time.Duration(c.TimeToLive) * time.Millisecond)

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
		InitFn:     c.InitFn,
		CloseFn:    c.CloseFn,
		TimeToLive: c.TimeToLive,
	}

	p.runConnection(newC)
}

func (p *ConnectionPoolWrapper) PrintConnectionMaps() {
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

		for _, h := range c.Handlers {
			h.cancelContext()
		}
		return true, nil
	}
	return false, nil
}

func (p *ConnectionPoolWrapper) GetConnectionHandler() (*ConnectionHandler, error) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	var ch ConnectionHandler
	c := p.GetConnection(p.ConnNum)
	ch.ConnectionWrapper = c

	clientConn := c.ClientConn.(*grpc.ClientConn)
	client := pb.NewChaincodeClient(clientConn)
	ctx, cancel := context.WithCancel(context.Background())
	ch.cancelContext = cancel

	//ctx := context.Background()
	stream, err := client.ChaincodeChat(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("chaincode chat failed on connection %v: %v", c.Id, err))
	}

	ch.chatClient = &stream

	c.Handlers = append(c.Handlers, &ch)

	return &ch, nil
}

func (ch *ConnectionHandler) SendReq(request *pb.ChaincodeRequest) error {
	ch.Mutex.Lock()
	defer ch.Mutex.Unlock()
	log.Printf(" | %v sending on connection: %v", request, ch.ConnectionWrapper.Id)
	ch.ConnectionWrapper.Requests++
	log.Printf("c.Request increased to: %v", ch.ConnectionWrapper.Requests)
	chatClient := *ch.chatClient
	if err := chatClient.Send(request); err != nil {
		return errors.New(fmt.Sprintf("failed to send request: %v", err))
	}
	return nil
}

func (ch *ConnectionHandler) RecvResp() (*pb.ChaincodeResponse, error) {
	ch.Mutex.Lock()
	defer ch.Mutex.Unlock()
	chatClient := *ch.chatClient
	in, err := chatClient.Recv()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to receive request: %v", err))
	}
	ch.ConnectionWrapper.Requests--
	log.Printf(" | %v recieved on connection: %v", in.Message, ch.ConnectionWrapper.Id)
	log.Printf("c.Request reduced to: %v", ch.ConnectionWrapper.Requests)
	return in, nil
}

func (ch *ConnectionHandler) SendResp(response *pb.ChaincodeResponse) error {
	ch.Mutex.Lock()
	defer ch.Mutex.Unlock()
	log.Printf(" | %v sending on connection: %v", response, ch.ConnectionWrapper.Id)
	ch.ConnectionWrapper.Requests++
	log.Printf("c.Request increased to: %v", ch.ConnectionWrapper.Requests)
	chatServer := *ch.chatServer
	if err := chatServer.Send(response); err != nil {
		return errors.New(fmt.Sprintf("failed to send request: %v", err))
	}
	return nil
}

func (ch *ConnectionHandler) RecvReq() (*pb.ChaincodeRequest, error) {
	ch.Mutex.Lock()
	defer ch.Mutex.Unlock()
	chatServer := *ch.chatServer
	in, err := chatServer.Recv()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to receive request: %v", err))
	}
	ch.ConnectionWrapper.Requests--
	log.Printf(" | %v recieved on connection: %v", in.Input, ch.ConnectionWrapper.Id)
	log.Printf("c.Request reduced to: %v", ch.ConnectionWrapper.Requests)
	return in, nil
}

func (ch *ConnectionHandler) CloseSend() error {
	ch.Mutex.Lock()
	defer ch.Mutex.Unlock()
	chatClient := *ch.chatClient
	err := chatClient.CloseSend()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to send close on stream: %v", err))
	}
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
