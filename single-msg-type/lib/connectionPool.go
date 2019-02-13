package lib

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/chainforce/free-peer/single-msg-type/chaincode_proto"
	"google.golang.org/grpc"
	"log"
	"sort"
	"sync"
	"time"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

const (
	address = "127.0.0.1:50051"
)

type ConnectionPool struct {
	sync.Mutex
	Size        int
	LiveConnMap map[int]*Connection
	DeadConnMap map[int]*Connection
	ConnNum     int
	Select		int
}

func (p *ConnectionPool) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	p.LiveConnMap = make(map[int]*Connection)
	p.DeadConnMap = make(map[int]*Connection)
	for i := 0; i < size; i++ {
		c := newConnection(p.ConnNum, ttL, initFn, closeFn)
		err := c.initConnection()
		if err != nil {
			return err
		}
		p.addConnection(c)
		go p.runConnection(c)
		time.Sleep(1000 * time.Millisecond)
	}
	p.Size = size
	return nil
}


func (p *ConnectionPool) runConnection(c *Connection) {
	p.PrintConnectionMaps()

	// Wait for Time to Live
	time.Sleep(time.Duration(c.TimeToLive) * time.Millisecond)

	// Kill connection
	c.Lock()
	if c.Requests == 0 {
		c.Unlock()
		err := c.killConnection()
		if err != nil {
			log.Fatalf("error killing connection: %v", err)
		}
	} else {
		c.Unlock()
		p.runConnection(c)
	}
	p.removeConnection(c)
	// Create new connection
	newC := newConnection(p.ConnNum, c.TimeToLive, c.InitFn, c.CloseFn)
	err := newC.initConnection()
	if err != nil {
		log.Printf("error starting connection: %v", err)
	}
	// Add connection
	p.addConnection(newC)
	// Run connection
	p.runConnection(newC)
}

func (p *ConnectionPool) addConnection(c *Connection) {
	p.Lock()
	defer p.Unlock()
	p.LiveConnMap[c.Id] = c
	p.ConnNum++
	return
}

func (p *ConnectionPool) PrintConnectionMaps() {
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

func (p *ConnectionPool) removeConnection(c *Connection) {
	log.Printf("1")
	p.Lock()
	log.Printf("2")
	defer p.Unlock()
	delete(p.LiveConnMap, c.Id)
	p.DeadConnMap[c.Id] = c
	return
}


func (p *ConnectionPool) GetConnection() (*Connection, error) {
	var ks []int
	for k := range p.LiveConnMap {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	log.Printf("ks: %v", ks)
	log.Printf("p.Select: %v", p.Select)
	log.Printf("p.ConnNum: %v", p.ConnNum)

	k := ks[p.Select]
	p.Select++

	if k == (p.ConnNum - 1) {
		p.Select = 0
	}

	c := p.LiveConnMap[k]
	return c, nil
}

func (p *ConnectionPool) GetConnectionHandler() (*ConnectionHandler, error) {
	p.Lock()
	defer p.Unlock()
	var h ConnectionHandler

	c, err := p.GetConnection()
	if err != nil {
		return nil, err
	}

	h.Connection = c

	clientConn := c.ClientConn.(*grpc.ClientConn)
	client := pb.NewChaincodeClient(clientConn)
	ctx := context.Background()
	stream, err := client.ChaincodeChat(ctx)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("chaincode chat failed on connection %v: %v", c.Id, err))
	}

	h.chatClient = &stream
	c.Handlers = append(c.Handlers, &h)

	return &h, nil
}


