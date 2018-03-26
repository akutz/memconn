package memconn

import (
	"net"
	"sync"
)

type memConn struct {
	addr memAddr
	chcn chan net.Conn
	done func(string) error
	pool *sync.Pool
}

func (m *memConn) Dial() (net.Conn, error) {
	r, w := pipe(m)
	go func() {
		m.chcn <- r
	}()
	return w, nil
}

func (m *memConn) Accept() (net.Conn, error) {
	for c := range m.chcn {
		return c, nil
	}
	return nil, ErrServerClosed
}

func (m *memConn) Close() error {
	return m.done(m.addr.addr)
}

func (m *memConn) Addr() net.Addr {
	return m.addr
}
