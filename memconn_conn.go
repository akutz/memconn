package memconn

import (
	"net"
	"time"
)

// Conn is an in-memory implementation of Golang's "net.Conn" interface.
type Conn struct {
	net.Conn
	localAddr  Addr
	remoteAddr Addr
}

// LocalAddr implements the net.Conn LocalAddr method.
func (c Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements the net.Conn RemoteAddr method.
func (c Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Read implements the net.Conn Read method.
func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.remoteAddr
			e.Source = c.localAddr
			return n, e
		}
		return n, err
	}
	return n, nil
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.remoteAddr
			e.Source = c.localAddr
			return n, e
		}
		return n, err
	}
	return n, nil
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	if err := c.Conn.SetReadDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.localAddr
			e.Source = c.localAddr
			return e
		}
		return err
	}
	return nil
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if err := c.Conn.SetWriteDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.localAddr
			e.Source = c.localAddr
			return e
		}
		return err
	}
	return nil
}
