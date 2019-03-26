package lib

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/chainforce/free-peer/app/chaincode_proto"
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
		p.Lock()
		p.ConnNum++
		p.Unlock()
		c := newConnection(p.ConnNum, ttL, initFn, closeFn)
		err := c.initConnection()
		if err != nil {
			return err
		}
		p.addConnection(c)
		go p.runConnection(c)
		//time.Sleep(1000 * time.Millisecond)
	}
	p.Size = size
	return nil
}


func (p *ConnectionPool) runConnection(c *Connection) {
	p.PrintConnectionMaps()

	// Wait for Time to Live
	time.Sleep(time.Duration(c.TimeToLive) * time.Millisecond)

	// Create new connection
	p.Lock()
	p.ConnNum++
	p.Unlock()
	newC := newConnection(p.ConnNum, c.TimeToLive, c.InitFn, c.CloseFn)
	err := newC.initConnection()
	if err != nil {
		log.Printf("error starting connection: %v", err)
	}
	// Add connection
	p.addConnection(newC)
	// Run connection
	go p.runConnection(newC)

	// Remove connection
	//c.Lock()
	p.removeConnection(c)
	//c.Unlock()
	// Kill connection
	alive := true
	for alive {
		log.Printf("Attempting to kill connection: %v with open requests: %v", c.Id, c.Requests)
		if c.Requests == 0 {
			err := c.killConnection()
			if err != nil {
				log.Fatalf("error killing connection: %v", err)
			}
			alive = false
		}
		time.Sleep(time.Second)
	}
	return
}

func (p *ConnectionPool) addConnection(c *Connection) {
	p.Lock()
	defer p.Unlock()
	p.LiveConnMap[c.Id] = c
	return
}

func (p *ConnectionPool) PrintConnectionMaps() {
	p.Lock()
	defer p.Unlock()
	var live []int
	for k := range p.LiveConnMap {
		live = append(live, k)
	}
	sort.Ints(live)
	log.Printf("Live connections: %v", live)

	var dead []int
	for k := range p.DeadConnMap {
		dead = append(dead, k)
	}
	sort.Ints(dead)
	log.Printf("Dead connections: %v", dead)
	return
}

func (p *ConnectionPool) removeConnection(c *Connection) {
	p.Lock()
	defer p.Unlock()
	delete(p.LiveConnMap, c.Id)
	p.DeadConnMap[c.Id] = c
	return
}


func (p *ConnectionPool) getConnection() (*Connection, error) {
	var ks []int
	for k := range p.LiveConnMap {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	log.Printf("ks: %v", ks)
	log.Printf("p.Select: %v", p.Select)
	log.Printf("p.ConnNum: %v", p.ConnNum)

	p.Select++
	if p.Select > (len(p.LiveConnMap) - 1) {
		p.Select = 0
	}

	k := ks[p.Select]

	/*if p.Select == (len(p.LiveConnMap)) {
		p.Select = 0
	}

	if k == p.ConnNum {
		p.Select = 0
	}*/

	c := p.LiveConnMap[k]
	return c, nil
}

func (p *ConnectionPool) GetConnectionHandler() (*ConnectionHandler, error) {
	p.Lock()
	defer p.Unlock()
	var h ConnectionHandler

	c, err := p.getConnection()
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


