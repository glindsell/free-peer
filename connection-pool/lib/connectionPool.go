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
	p.ConnChanOpen = make(chan *ConnectionWrapper, size)
	p.TimeToLive = ttL
	for i := 0; i < size; i++ {
		c := p.NewConnection(i, ttL, initFn, closeFn)
		go p.RunConnection(c)
	}
	p.Size = size
	go p.ManageConnections()
	return nil
}

func (p *ConnectionPoolWrapper) NewConnection(id, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *ConnectionWrapper {
	c := &ConnectionWrapper{}
	c.Id = id
	c.InitFn = initFn
	c.CloseFn = closeFn
	return c
}

func (p *ConnectionPoolWrapper) GetConnection() *ConnectionWrapper {
	c := <-p.ConnChanOpen
	return c
}

func (p *ConnectionPoolWrapper) ReleaseConnection(c *ConnectionWrapper) {
	p.ConnChanOpen <- c
	return
}

func (p *ConnectionPoolWrapper) RunConnection(c *ConnectionWrapper) {
	err := c.InitConnection()
	if err != nil {
		log.Fatalf("error starting connection: %v", err)
	}
	p.OpenConn++
	p.ConnChanOpen <- c
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
	log.Printf("Started connection: %v", strconv.Itoa(c.Id))
	return nil
}

func (c *ConnectionWrapper) CloseConnection() error {
	log.Printf("Closing connection: %v", c.Id)
		err := c.CloseFn(c.ClientConn)
		if err != nil {
			return errors.New(fmt.Sprintf("error closing connection: %v", err))
		}
	log.Printf("Closed connection: %v", c.Id)
	return nil
}

func (p *ConnectionPoolWrapper) ManageConnections() {
	for {
		// Wait for Time to Live
		time.Sleep(time.Duration(p.TimeToLive) * time.Second)
		log.Printf("Getting connection to reset...")
		c := p.GetConnection()
		log.Printf("Resetting connection: %v", c.Id)

		// Reset connection
		err := c.ResetConnection()
		if err != nil {
			log.Fatalf("error resetting connection: %v", err)
		}
		p.ReleaseConnection(c)
	}
}
