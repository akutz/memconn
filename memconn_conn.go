package memconn

import (
	"net"
	"net/http"
	"sync"
)

type memconn struct {
	sync.Once
	addr addr
	bsiz uint64
	chcn chan net.Conn
	done func()
	pool *sync.Pool
}

func (m *memconn) Dial() (net.Conn, error) {
	r, w := pipe(&m.addr, m.pool)
	go func() {
		m.chcn <- r
	}()
	return w, nil
}

func (m *memconn) Accept() (net.Conn, error) {
	for c := range m.chcn {
		return c, nil
	}
	return nil, http.ErrServerClosed
}

func (m *memconn) Close() error {
	// The close logic is executed exactly once. Additional calls to
	// Close immediately return http.ErrServerClosed.
	m.Do(func() {
		close(m.chcn)
		m.done()
	})
	return http.ErrServerClosed
}

func (m *memconn) Addr() net.Addr {
	return m.addr
}
