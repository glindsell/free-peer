package lib

import (
	"log"
	"strconv"
	"time"
)

type InitClientConnFunction func() (interface{}, error)
type CloseClientConnFunction func(interface{}) error

type ConnectionPoolWrapper struct {
	size int
	connChan chan *ConnectionWrapper
}

type ConnectionWrapper struct {
	Id string
	ClientConn interface{}
	InFlight bool
	timeToLive int
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
	p.connChan = make(chan *ConnectionWrapper, size)
	for x := 0; x < size; x++ {
		log.Println("Connection " + strconv.Itoa(x) + " starting")
		conn := &ConnectionWrapper{}
		go conn.ManageConnection(x, ttL, initFn, closeFn)

		// Add the connection to the channel
		p.connChan <- conn
	}
	p.size = size
	return nil
}

func (p *ConnectionPoolWrapper) GetConnection() *ConnectionWrapper {
	return <-p.connChan
}

func (p *ConnectionPoolWrapper) ReleaseConnection(conn *ConnectionWrapper) {
	p.connChan <- conn
}

func (p *ConnectionPoolWrapper) ManageConnections() {
	//TODO: This is where code which keeps correct number of connections alive will go
}

func (c *ConnectionWrapper) ManageConnection(id int, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) {
	// Start connection
	err := c.InitConnection(id, ttL, initFn)
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}

	// Wait for Time to Live
	time.Sleep(time.Duration(ttL) * time.Second)

	// Close connection
	err = c.CloseConnection(closeFn)
	if err != nil {
		log.Fatalf("error closing connection: %v", err)
	}
	return
}

func (c *ConnectionWrapper) InitConnection(id int, ttL int, initFn InitClientConnFunction) error {
	clientConn, err := initFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	c.Id = strconv.Itoa(id)
	c.timeToLive = ttL
	return nil
}

func (c *ConnectionWrapper) CloseConnection(closefn CloseClientConnFunction) error {
	log.Printf("Closing connection: %v", c.Id)
	return closefn(c.ClientConn)
}