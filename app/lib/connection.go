package lib

import (
	"log"
	"strconv"
	"sync"
)

type Connection struct {
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

func newConnection(id int, ttL int, initFn InitClientConnFunction, closeFn CloseClientConnFunction) *Connection {
	c := &Connection{
		Id: id,
		InitFn: initFn,
		CloseFn: closeFn,
		TimeToLive: ttL,
	}
	return c
}

func (c *Connection) killConnection() error {
	c.Lock()
	defer c.Unlock()
	err := c.CloseFn(c.ClientConn)
	if err != nil {
		return err
	}
	c.Dead = true
	log.Printf("Killed connection: %v", strconv.Itoa(c.Id))
	return nil
}

func (c *Connection) initConnection() error {
	c.Lock()
	defer c.Unlock()
	log.Printf("Starting connection: %v", strconv.Itoa(c.Id))
	clientConn, err := c.InitFn()
	if err != nil {
		return err
	}
	c.ClientConn = clientConn
	log.Printf("Started connection: %v", strconv.Itoa(c.Id))
	return nil
}
