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
	Size       int
	ConnChan   chan *ConnectionWrapper
	TimeToLive int
}

type ConnectionWrapper struct {
	Id int
	ClientConn interface{}
	InitFn InitClientConnFunction
	CloseFn CloseClientConnFunction
}

func (p *ConnectionPoolWrapper) InitPool(size, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) error {
	// Create a buffered channel allowing size senders
	p.ConnChan = make(chan *ConnectionWrapper, size)
	p.TimeToLive = ttL
	for i := 0; i < size; i++ {
		c := p.newConnection(i, ttL, initFn, closeFn)
		p.addConnection(c)
	}
	p.Size = size
	go p.manageConnections()
	return nil
}

func (p *ConnectionPoolWrapper) GetConnection() *ConnectionWrapper {
	c := <-p.ConnChan
	return c
}

func (p *ConnectionPoolWrapper) ReleaseConnection(c *ConnectionWrapper) {
	p.ConnChan <- c
	return
}

func (p *ConnectionPoolWrapper) newConnection(id, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *ConnectionWrapper {
	c := &ConnectionWrapper{}
	c.Id = id
	c.InitFn = initFn
	c.CloseFn = closeFn
	return c
}

func (p *ConnectionPoolWrapper) addConnection(c *ConnectionWrapper) {
	err := c.initConnection()
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}
	p.ConnChan <- c
	return
}

func (c *ConnectionWrapper) resetConnection() error {
	err := c.closeConnection()
	if err != nil {
		return errors.New(fmt.Sprintf("error closing connection: %v", err))
	}
	err = c.initConnection()
	if err != nil {
		return errors.New(fmt.Sprintf("error starting connection: %v", err))
	}
	return nil
}

func (c *ConnectionWrapper) initConnection() error {
	log.Printf("Starting connection: %v", strconv.Itoa(c.Id))
	clientConn, err := c.InitFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	log.Printf("Started connection: %v", strconv.Itoa(c.Id))
	return nil
}

func (c *ConnectionWrapper) closeConnection() error {
	log.Printf("Closing connection: %v", c.Id)
		err := c.CloseFn(c.ClientConn)
		if err != nil {
			return errors.New(fmt.Sprintf("error closing connection: %v", err))
		}
	log.Printf("Closed connection: %v", c.Id)
	return nil
}

func (p *ConnectionPoolWrapper) manageConnections() {
	for {
		// Wait for Time to Live
		time.Sleep(time.Duration(p.TimeToLive) * time.Millisecond)
		log.Printf("Getting connection to reset...")
		c := p.GetConnection()
		log.Printf("Resetting connection: %v", c.Id)

		// Reset connection
		err := c.resetConnection()
		if err != nil {
			log.Fatalf("error resetting connection: %v", err)
		}
		p.ReleaseConnection(c)
	}
}
