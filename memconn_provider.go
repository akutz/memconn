package memconn

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// Provider is used to track named MemConn objects.
type Provider struct {
	sync.Once
	sync.RWMutex
	chanPool sync.Pool
	cnxns    map[string]MemConn
}

var provider = &Provider{}

// Listen creates a new, named MemConn and stores it in the provider.
// If a MemConn with the specified name already exists then an error is
// returned.
// The network type returned by MemConn.Addr().Network() is "memconn".
// The provided name is returned by MemConn.Addr().String().
func (p *Provider) Listen(name string) (net.Listener, error) {
	p.Once.Do(func() { p.cnxns = map[string]MemConn{} })

	p.Lock()
	defer p.Unlock()

	if _, ok := p.cnxns[name]; ok {
		return nil, fmt.Errorf("memconn: name in use: %s", name)
	}

	c := &memconn{
		addr: addr{name: name},
		chcn: make(chan net.Conn, 1),
		done: func() {
			p.Lock()
			defer p.Unlock()
			delete(p.cnxns, name)
		},
		pool: &p.chanPool,
	}

	p.cnxns[name] = c
	return c, nil
}

// Dial dials a named PipeConn in the connection pool and returns the
// net.Conn object if the connection is successful.
func (p *Provider) Dial(name string) (net.Conn, error) {
	p.RLock()
	defer p.RUnlock()

	c, ok := p.cnxns[name]
	if !ok {
		return nil, fmt.Errorf("memconn: invalid name: %s", name)
	}
	return c.Dial()
}

// DialHTTP dials a named PipeConn in the connection pool and returns an
// http.Client object if the connection is successful. The client's
// Transport field is set to a *http.Transport with a custom DialContext
// function that uses this package's Dial function to access the named
// pipe.
func (p *Provider) DialHTTP(name string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(
				context.Context, string, string) (net.Conn, error) {
				conn, err := Dial(name)
				if err != nil {
					return nil, err
				}
				return conn, nil
			},
		},
	}
}

// Drain removes all of the channels from the channel pool.
func (p *Provider) Drain() {
	for {
		if p.chanPool.Get() == nil {
			break
		}
	}
}
