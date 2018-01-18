package memconn

type addr struct {
	name string
}

const network = "memconn"

func (a addr) Network() string {
	return network
}

func (a addr) String() string {
	return a.name
}
