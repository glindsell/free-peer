package lib

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/chainforce/free-peer/multi-request/chaincode_proto"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

const (
	address = "127.0.0.1:50051"
	port = ":50052"
)

type ConnectionPoolWrapper struct {
	sync.Mutex
	Size        int
	LiveConnMap map[int]*ConnectionWrapper
	DeadConnMap map[int]*ConnectionWrapper
	ConnNum     int
	Select      int
}

type ConnectionWrapper struct {
	sync.Mutex
	Id         int
	ClientConn interface{}
	InitFn     InitClientConnFunction
	CloseFn    CloseClientConnFunction
	TimeToLive int
	Requests   int
	Dead       bool
	Handlers   []*ConnectionHandler
}

type ConnectionHandler struct {
	sync.Mutex
	ConnectionWrapper *ConnectionWrapper
	clientStream      pb.ChatService_ChatClient // allows gRPC stream
	serverStream      pb.ChatService_ChatServer // allows gRPC stream
	cancelContext     context.CancelFunc
}

func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	p.LiveConnMap = make(map[int]*ConnectionWrapper)
	p.DeadConnMap = make(map[int]*ConnectionWrapper)
	for i := 0; i < size; i++ {
		c := p.newConnection(ttL, initFn, closeFn)
		err := p.initConnection(c)
		if err != nil {
			return err
		}
		//go p.runConnection(c)
		time.Sleep(1000 * time.Millisecond)
	}
	p.Size = size
	return nil
}

func (p *ConnectionPoolWrapper) GetConnection(k int) *ConnectionWrapper {
	c := p.LiveConnMap[k]
	c.Requests++
	return c
}

func (p *ConnectionPoolWrapper) ReleaseConnection(k int) {
	c := p.LiveConnMap[k]
	c.Requests--
}

func (p *ConnectionPoolWrapper) newConnection(ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *ConnectionWrapper {
	c := &ConnectionWrapper{}
	c.InitFn = initFn
	c.CloseFn = closeFn
	c.TimeToLive = ttL
	return c
}

func (p *ConnectionPoolWrapper) runConnection(c *ConnectionWrapper) {
	p.PrintConnectionMaps()

	// Wait for Time to Live
	time.Sleep(time.Duration(c.TimeToLive) * time.Millisecond)

	// Reset connection
	err := p.resetConnection(c)
	if err != nil {
		log.Fatalf("error killing connection: %v", err)
	}
}

func (p *ConnectionPoolWrapper) PrintConnectionMaps() {
	p.Lock()
	defer p.Unlock()
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
	return
}

func (p *ConnectionPoolWrapper) initConnection(c *ConnectionWrapper) error {
	p.Lock()
	defer p.Unlock()
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

func (p *ConnectionPoolWrapper) resetConnection(c *ConnectionWrapper) error {
	p.Lock()
	defer p.Unlock()
	c.Lock()
	defer c.Unlock()
	// Kill connection
	log.Printf("Killing connection: %v", strconv.Itoa(c.Id))
	killed, err := p.killConnection(c)
	if err != nil {
		return errors.New(fmt.Sprintf("error killing connection: %v", err))
	}
	for killed != true {
		log.Printf("failed to kill connection, connection in use, retrying later")
		time.Sleep(1 * time.Second)
		killed, err = p.killConnection(c)
		if err != nil {
			return errors.New(fmt.Sprintf("error killing connection: %v", err))
		}
	}

	// Create new connection
	newC := p.newConnection(c.TimeToLive, c.InitFn, c.CloseFn)
	err = p.initConnection(newC)
	if err != nil {
		return errors.New(fmt.Sprintf("error starting connection: %v", err))
	}
	// Run new connection
	go p.runConnection(newC)

	return nil
}

func (p *ConnectionPoolWrapper) killConnection(c *ConnectionWrapper) (bool, error) {
	if c.Requests == 0 {
		delete(p.LiveConnMap, c.Id)
		err := c.CloseFn(c.ClientConn)
		if err != nil {
			return false, err
		}
		c.Dead = true
		p.DeadConnMap[c.Id] = c
		log.Printf("Killed connection: %v", strconv.Itoa(c.Id))

		/*for _, h := range c.Handlers {
			err = h.CloseSend()
			if err != nil {
				return false, err
			}
			h.cancelContext()
		}*/
	}
	return false, nil

}

func (p *ConnectionPoolWrapper) getAllConnections() []int {
	var ks []int
	for k := range p.LiveConnMap {
		ks = append(ks, k)
	}
	return ks
}

func (p *ConnectionPoolWrapper) GetConnectionHandler() (*ConnectionHandler, error) {
	p.Lock()
	defer p.Unlock()

	var err error
	var ch ConnectionHandler

	ks := p.getAllConnections()
	sort.Ints(ks)

	k := ks[p.Select]
	p.Select++

	if k == p.ConnNum {
		p.Select = 0
	}

	c := p.GetConnection(k)

	ch.ConnectionWrapper = c

	clientConn := c.ClientConn.(*grpc.ClientConn)
	client := pb.NewChatServiceClient(clientConn)

	ctx, cancel := context.WithCancel(context.Background())
	ch.cancelContext = cancel

	ch.clientStream, err = client.Chat(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("chaincode chat failed on connection %v: %v", c.Id, err))
	}

	c.Handlers = append(c.Handlers, &ch)

	return &ch, nil
}

func Serve() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	serv := newChatServer()
	pb.RegisterChatServiceServer(s, serv)
	log.Println("Peer: " + serv.chatServerName + " started.")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
func newChatServer() *chatServer {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := &chatServer{chatServerName: strconv.Itoa(r.Int())}
	return s
}

type chatServer struct {
	chatServerName string
}

func (c *chatServer) Chat(server pb.ChatService_ChatServer) error {
	// Do something with peer as a server!
	req, err := server.Recv()
	if err != nil {
		return err
	}
	log.Printf("Req received on chat server: %v", req)
	return nil
}

func (h *ConnectionHandler) SendReq(request *pb.ChatRequest) error {
	h.Lock()
	defer h.Unlock()
	h.ConnectionWrapper.Requests++
	log.Printf(" | SEND >>> %v on connection: %v", request.Input, h.ConnectionWrapper.Id)
	if err := h.clientStream.Send(request); err != nil {
		return errors.New(fmt.Sprintf("failed to send request: %v", err))
	}
	return nil
}

func (h *ConnectionHandler) RecvResp() (*pb.ChatResponse, error) {
	h.Lock()
	defer h.Unlock()
	h.ConnectionWrapper.Requests--
	in, err := h.clientStream.Recv()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to receive request: %v", err))
	}
	log.Printf(" | REVC <<< %v on connection: %v", in.Message, h.ConnectionWrapper.Id)
	return in, nil
}

func (h *ConnectionHandler) SendResp(response *pb.ChatResponse) error {
	h.Lock()
	defer h.Unlock()
	h.ConnectionWrapper.Requests++
	log.Printf(" | SEND >>> %v on connection: %v", response.Message, h.ConnectionWrapper.Id)
	if err := h.serverStream.Send(response); err != nil {
		return errors.New(fmt.Sprintf("failed to send request: %v", err))
	}
	return nil
}

func (h *ConnectionHandler) RecvReq() (*pb.ChatRequest, error) {
	h.Lock()
	defer h.Unlock()
	h.ConnectionWrapper.Requests--
	in, err := h.serverStream.Recv()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to receive request: %v", err))
	}
	log.Printf(" | RECV <<< %v on connection: %v", in.Input, h.ConnectionWrapper.Id)
	return in, nil
}

func (h *ConnectionHandler) closeSend() error {
	h.Lock()
	defer h.Unlock()
	err := h.clientStream.CloseSend()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to send close on stream: %v", err))
	}
	return nil
}

func (h *ConnectionHandler) Done(txID int32) error {
	err := h.closeSend()
	if err != nil {
		return err
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
		log.Fatalf("did not close: %v", err)
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
