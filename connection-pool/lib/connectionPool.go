package lib

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

type ConnectionPoolWrapper struct {
	Size         int
	OpenConn     int
	ConnChanOpen chan *ConnectionWrapper
	//mux sync.Mutex
}

type ConnectionWrapper struct {
	Id int
	ClientConn interface{}
	InFlight bool
	TimeToLive int
	InitFn InitClientConnFunction
	CloseFn CloseClientConnFunction
	Closed bool
	//mux sync.Mutex
	//handler chaincode.Handler
}

/**
 Call the init function size times. If the init function fails during any call, then
 the creation of the pool is considered a failure.
 We call the same function size times to make sure each connection shares the same
 state.
*/
func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	go func() {
		for {
			log.Printf("Conn: %v", p.OpenConn)
			time.Sleep(100 * time.Millisecond)
		}
	}()
	// Create a buffered channel allowing size senders
	p.ConnChanOpen = make(chan *ConnectionWrapper, size)
	for x := 0; x < size; x++ {
		//id := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
		c := p.NewConnection(x, ttL, initFn, closeFn)
		go p.RunConnection(c)
	}
	p.Size = size
	time.Sleep(time.Second)
	//go p.ManageConnections(ttL, initFn, closeFn)
	return nil
}

func (p *ConnectionPoolWrapper) NewConnection(id, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *ConnectionWrapper {
	c := &ConnectionWrapper{}
	c.Id = id
	c.TimeToLive = ttL
	c.InitFn = initFn
	c.CloseFn = closeFn
	return c
}

func (p *ConnectionPoolWrapper) GetConnection() *ConnectionWrapper {
	c := <-p.ConnChanOpen
	c.InFlight = true
	return c
}

func (p *ConnectionPoolWrapper) ReleaseConnection(c *ConnectionWrapper) {
	p.ConnChanOpen <- c
	c.InFlight = false
	return
}

func (p *ConnectionPoolWrapper) RunConnection(c *ConnectionWrapper) {
	err := c.InitConnection()
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}
	p.OpenConn++
	p.ConnChanOpen <- c

	for {
		// Wait for Time to Live
		time.Sleep(time.Duration(c.TimeToLive) * time.Second)

		// Reset connection
		err = c.ResetConnection()
		if err != nil {
			log.Fatalf("error starting connection: %v", err)
		}
	}
}

func (c *ConnectionWrapper) ResetConnection() error {
	err := c.CloseConnection()
	if err != nil {
		return errors.New(fmt.Sprintf("error closing connection: %v", err))
	}
	err = c.InitConnection()
	if err != nil {
		return errors.New(fmt.Sprintf("error starting connection: %v", err))
	}
	return nil
}

func (c *ConnectionWrapper) InitConnection() error {
	log.Printf("Starting connection: %v", strconv.Itoa(c.Id))
	clientConn, err := c.InitFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	c.Closed = false
	log.Printf("Started connection: %v", strconv.Itoa(c.Id))
	return nil
}

func (c *ConnectionWrapper) CloseConnection() error {
	log.Printf("Closing connection: %v", c.Id)
	if c.InFlight != true {
		err := c.CloseFn(c.ClientConn)
		if err != nil {
			return errors.New(fmt.Sprintf("error closing connection: %v", err))
		}
	} else {
		log.Printf("Connection %v not cancelled, connection in flight, retrying...", c.Id)
		err := c.CloseConnection()
		if err != nil {
			return errors.New(fmt.Sprintf("error closing connection: %v", err))
		}
	}
	c.Closed = true
	log.Printf("Closed connection: %v", c.Id)
	return nil
}

/*func (p *ConnectionPoolWrapper) ManageConnections(ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) {
	for {
		if p.OpenConn < p.Size {
			id := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
			c := &ConnectionWrapper{
				Id: id,
				TimeToLive: ttL,
				InitFn: initFn,
				CloseFn: closeFn,
			}
			go p.RunConnection(c)
		}
		time.Sleep(time.Nanosecond)
	}
}*/
