package memconn

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ErrTimeout occurs when a read or write operation times out with respect
// to the connection's deadline settings.
var ErrTimeout = errors.New("i/o timeout")

func pipe(addr *addr, pool *sync.Pool) (net.Conn, net.Conn) {
	c1, c2 := net.Pipe()
	return &pipeConn{
			Conn: c1,
			addr: addr,
			pool: pool,
		}, &pipeConn{
			Conn: c2,
			addr: addr,
			pool: pool,
		}
}

type pipeConn struct {
	net.Conn
	addr *addr
	pool *sync.Pool
	rd   time.Time
	wd   time.Time
}

func (p *pipeConn) SetDeadline(t time.Time) error {
	p.SetReadDeadline(t)
	p.SetWriteDeadline(t)
	return nil
}

func (p *pipeConn) SetReadDeadline(t time.Time) error {
	if !t.Before(time.Now()) {
		p.rd = t
	}
	return nil
}

func (p *pipeConn) SetWriteDeadline(t time.Time) error {
	if !t.Before(time.Now()) {
		p.wd = t
	}
	return nil
}

const (
	opRead  = "read"
	opWrite = "write"
)

func (p *pipeConn) Read(data []byte) (int, error) {
	return p.doReadOrWrite(opRead, data)
}

func (p *pipeConn) Write(data []byte) (int, error) {
	return p.doReadOrWrite(opWrite, data)
}

func (p *pipeConn) doReadOrWrite(op string, data []byte) (int, error) {

	deadline := p.rd
	if op == opWrite {
		deadline = p.wd
	}

	// If there is no deadline then bypass the timeout logic.
	if deadline.IsZero() {
		if op == opRead {
			return p.Conn.Read(data)
		}
		return p.Conn.Write(data)
	}

	// Create a timer only if the deadline is not zero and does
	// not occur before the current time.
	timeout := time.NewTimer(deadline.Sub(time.Now()))
	defer timeout.Stop()

	c, ok := p.pool.Get().(chan interface{})
	if !ok {
		// The channel must be buffered with a length of 1 or else
		// the send operations in the below goroutine will block
		// forever, preventing the goroutine from exiting, if the
		// IO operation times out.
		c = make(chan interface{}, 1)
	}

	go func() {
		var n int
		var err error
		if op == opRead {
			n, err = p.Conn.Read(data)
		} else {
			n, err = p.Conn.Write(data)
		}
		if err != nil {
			c <- err
		} else {
			c <- n
		}
	}()

	select {
	case <-timeout.C:
		return 0, &net.OpError{
			Op:     op,
			Net:    network,
			Source: p.addr,
			Addr:   p.addr,
			Err:    ErrTimeout,
		}
	case i := <-c:
		p.pool.Put(c)
		switch ti := i.(type) {
		case int:
			return ti, nil
		case error:
			return 0, ti
		default:
			panic(fmt.Sprintf("memconn: invalid type: %[1]T %[1]v", i))
		}
	}
}
