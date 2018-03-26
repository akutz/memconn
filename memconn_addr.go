package memconn

type memAddr struct {
	network string
	addr    string
}

func (m memAddr) Network() string {
	return m.network
}

func (m memAddr) String() string {
	return m.addr
}
