package memconn

import (
	"fmt"
	"net"
	"sync"
)

// Provider is used to track named MemConn objects.
type Provider struct {
	sync.Once
	sync.RWMutex
	chanPool sync.Pool
	cnxns    map[string]*memConn
}

var provider = Provider{}

// Listen creates a new, named MemConn and stores it in the provider.
// Known networks are "memu" (memconn unbuffered).
// If a MemConn with the specified address already exists then an error is
// returned.
func (p *Provider) Listen(network, addr string) (net.Listener, error) {
	p.Once.Do(func() { p.cnxns = map[string]*memConn{} })

	p.Lock()
	defer p.Unlock()

	if _, ok := p.cnxns[addr]; ok {
		return nil, fmt.Errorf("memconn: addr in use: %s", addr)
	}

	c := &memConn{
		addr: memAddr{network: network, addr: addr},
		chcn: make(chan net.Conn, 1),
		done: p.closeConn,
		pool: &p.chanPool,
	}

	p.cnxns[addr] = c
	return c, nil
}

// Dial dials a named MemConn that belongs to this provider and returns the
// net.Conn object if the connection is successful.
// Known networks are "memu" (memconn unbuffered).
func (p *Provider) Dial(_, name string) (net.Conn, error) {
	p.Once.Do(func() { p.cnxns = map[string]*memConn{} })

	p.RLock()
	defer p.RUnlock()

	c, ok := p.cnxns[name]
	if !ok {
		return nil, fmt.Errorf("memconn: invalid name: %s", name)
	}
	return c.Dial()
}

// Drain removes all of the channels from the channel pool.
func (p *Provider) Drain() {
	for p.chanPool.Get() != nil {
		// Drain the pool
	}
}

// Prime initializes the provider with n channels.
func (p *Provider) Prime(n int) {
	for i := 0; i < n; i++ {
		p.chanPool.Put(make(chan interface{}, 1))
	}
}

func (p *Provider) closeConn(name string) error {
	p.Lock()
	defer p.Unlock()
	if c, ok := p.cnxns[name]; ok {
		delete(p.cnxns, name)
		close(c.chcn)
	}
	return ErrServerClosed
}
