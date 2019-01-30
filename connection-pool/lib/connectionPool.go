package lib

import (
	"log"
	"math/rand"
	"strconv"
	"time"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

type ConnectionPoolWrapper struct {
	Size int
	OpenConn int
	ConnChan chan *ConnectionWrapper
}

type ConnectionWrapper struct {
	Id int
	ClientConn interface{}
	InFlight bool
	TimeToLive int
	InitFn InitClientConnFunction
	CloseFn CloseClientConnFunction
	//handler chaincode.Handler
}

/**
 Call the init function size times. If the init function fails during any call, then
 the creation of the pool is considered a failure.
 We call the same function size times to make sure each connection shares the same
 state.
*/
func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	// Create a buffered channel allowing size senders
	p.ConnChan = make(chan *ConnectionWrapper, size)
	for x := 0; x < size; x++ {
		id := rand.New(rand.NewSource(time.Now().UnixNano()))
		log.Println("Connection " + strconv.Itoa(id.Int()) + " starting")
		conn := &ConnectionWrapper{}
		go conn.ManageConnection(id.Int(), ttL, initFn, closeFn, p)

		// Add the connection to the channel
		p.ConnChan <- conn
		p.OpenConn++
	}
	p.Size = size
	conn := &ConnectionWrapper{
		TimeToLive: ttL,
		InitFn: initFn,
		CloseFn: closeFn,
	}
	time.Sleep(time.Second)
	go p.ManageConnections(conn)
	return nil
}

func (p *ConnectionPoolWrapper) GetConnection() *ConnectionWrapper {
	conn := <-p.ConnChan
	conn.InFlight = true
	return conn
}

func (p *ConnectionPoolWrapper) ReleaseConnection(conn *ConnectionWrapper) {
	conn.InFlight = false
	p.ConnChan <- conn
}

func (p *ConnectionPoolWrapper) ManageConnections(conn *ConnectionWrapper) {
	for {
		if p.OpenConn < p.Size {
			id := rand.New(rand.NewSource(time.Now().UnixNano()))
			conn.ManageConnection(id.Int(), conn.TimeToLive, conn.InitFn, conn.CloseFn, p)
			p.ConnChan <- conn
			p.OpenConn++
		}
		time.Sleep(time.Nanosecond)
	}
}

func (c *ConnectionWrapper) ManageConnection(id int, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction, pool *ConnectionPoolWrapper) {
	// Start connection
	err := c.InitConnection(id, ttL, initFn, closeFn)
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}

	// Wait for Time to Live
	time.Sleep(time.Duration(ttL) * time.Second)

	if c.InFlight != true {
		// Close connection
		err = c.CloseConnection(closeFn)
		if err != nil {
			log.Fatalf("error closing connection: %v", err)
		}
		pool.OpenConn--
	} else {
		log.Printf("Connection not cancelled, connection in flight")
	}
	return
}

func (c *ConnectionWrapper) InitConnection(id int, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	clientConn, err := initFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	c.Id = id
	c.TimeToLive = ttL
	c.InitFn = initFn
	c.CloseFn = closeFn
	return nil
}

func (c *ConnectionWrapper) CloseConnection(closefn CloseClientConnFunction) error {
	log.Printf("Closing connection: %v", c.Id)
	return closefn(c.ClientConn)
}