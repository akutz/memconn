# MemConn
MemConn is a named, in-memory network connection for Go that supports
deadlines.

## Create a Server
A new `net.Listener` used to serve HTTP, gRPC, etc. is created with
`memconn.Listen`:

```go
lis, err := memconn.Listen("memu", "UniqueName")
```

## Creating a Client (Dial)
Clients can dial any named connection:

```go
client, err := memconn.Dial("memu", "UniqueName")
```

## Network Types
MemCon supports the following network types:

| Network | Description |
|---------|-------------|
| `memu`  | An unbuffered, in-memory implementation of `net.Conn` |

## Performance
The benchmark results illustrates MemConn's performance against TCP
and UNIX domain sockets:

```shell
$ go test -benchmem -bench Benchmark -run Benchmark -v
goos: darwin
goarch: amd64
pkg: github.com/akutz/memconn
BenchmarkMemu-8   	 1000000	      1503 ns/op	    1385 B/op	      15 allocs/op
--- BENCH: BenchmarkMemu-8
	memconn_test.go:238: serving memu:1522282533513414735
	memconn_test.go:238: serving memu:1522282533513962026
	memconn_test.go:238: serving memu:1522282533514763754
	memconn_test.go:238: serving memu:1522282533530642645
BenchmarkTCP-8    	   50000	     49890 ns/op	     902 B/op	      20 allocs/op
--- BENCH: BenchmarkTCP-8
	memconn_test.go:238: serving tcp:127.0.0.1:60762
	memconn_test.go:238: serving tcp:127.0.0.1:60764
	memconn_test.go:238: serving tcp:127.0.0.1:60865
	memconn_test.go:238: serving tcp:127.0.0.1:54504
BenchmarkUNIX-8   	   50000	     23619 ns/op	    1425 B/op	      20 allocs/op
--- BENCH: BenchmarkUNIX-8
	memconn_test.go:238: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/594086618.sock
	memconn_test.go:238: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/606000241.sock
	memconn_test.go:238: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/265318684.sock
	memconn_test.go:238: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/253179339.sock
PASS
ok  	github.com/akutz/memconn	5.840s
```

MemConn is faster and allocates fewer objects than the TCP and UNIX domain
sockets. While MemConn does allocate more memory, this is to be expected
since MemConn is an in-memory implementation of the `net.Conn` interface.
