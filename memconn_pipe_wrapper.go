package memconn

import (
	"net"
	"time"
)

type pipeWrapper struct {
	net.Conn
	localAddr  Addr
	remoteAddr Addr
}

func (p pipeWrapper) LocalAddr() net.Addr {
	return p.localAddr
}

func (p pipeWrapper) RemoteAddr() net.Addr {
	return p.remoteAddr
}

func (p *pipeWrapper) Read(b []byte) (int, error) {
	n, err := p.Conn.Read(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = p.remoteAddr
			e.Source = p.localAddr
			return n, e
		}
		return n, err
	}
	return n, nil
}

func (p *pipeWrapper) Write(b []byte) (int, error) {
	n, err := p.Conn.Write(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = p.remoteAddr
			e.Source = p.localAddr
			return n, e
		}
		return n, err
	}
	return n, nil
}

func (p *pipeWrapper) SetReadDeadline(t time.Time) error {
	if err := p.Conn.SetDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = p.localAddr
			e.Source = p.localAddr
			return e
		}
		return err
	}
	return nil
}

func (p *pipeWrapper) SetWriteDeadline(t time.Time) error {
	if err := p.Conn.SetWriteDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = p.localAddr
			e.Source = p.localAddr
			return e
		}
		return err
	}
	return nil
}
