package memconn

import (
	"errors"
	"net"
	"time"
)

// ErrTimeout occurs when a read or write operation times out with respect
// to the connection's deadline settings.
var ErrTimeout = errors.New("i/o timeout")

func pipe(conn *memConn) (net.Conn, net.Conn) {
	c1, c2 := net.Pipe()
	return &pipeConn{
			Conn: c1,
			conn: conn,
		}, &pipeConn{
			Conn: c2,
			conn: conn,
		}
}

type pipeConn struct {
	net.Conn
	conn *memConn
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

type opType uint8

const (
	opRead opType = iota
	opWrite
)

func (s opType) String() string {
	if s == opRead {
		return "read"
	}
	return "write"
}

func (p *pipeConn) Read(data []byte) (int, error) {
	return p.doReadOrWrite(opRead, data)
}

func (p *pipeConn) Write(data []byte) (int, error) {
	return p.doReadOrWrite(opWrite, data)
}

func (p *pipeConn) doReadOrWrite(op opType, data []byte) (int, error) {

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

	c, ok := p.conn.pool.Get().(chan ioResult)
	if !ok {
		// The channel must be buffered with a length of 1 or else
		// the send operations in the below goroutine will block
		// forever, preventing the goroutine from exiting, if the
		// IO operation times out.
		c = make(chan ioResult, 1)
	}

	go func() {
		var r ioResult
		if op == opRead {
			r.n, r.err = p.Conn.Read(data)
		} else {
			r.n, r.err = p.Conn.Write(data)
		}
		c <- r
	}()

	select {
	case <-timeout.C:
		return 0, &net.OpError{
			Op:     op.String(),
			Net:    p.conn.addr.Network(),
			Source: p.conn.addr,
			Addr:   p.conn.addr,
			Err:    ErrTimeout,
		}
	case r := <-c:
		p.conn.pool.Put(c)
		return r.n, r.err
	}
}
