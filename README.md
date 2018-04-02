# MemConn
MemConn provides named, in-memory network connections for Go.

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
$ make benchmark
go test -bench . -run Bench -benchmem .
goos: darwin
goarch: amd64
pkg: github.com/akutz/memconn
BenchmarkMemu-8               	  500000	      3216 ns/op	    1449 B/op	      19 allocs/op
--- BENCH: BenchmarkMemu-8
	memconn_test.go:239: serving memu:1522710150158956844
	memconn_test.go:239: serving memu:1522710150159779641
	memconn_test.go:239: serving memu:1522710150160660752
	memconn_test.go:239: serving memu:1522710150193395496
BenchmarkMemuWithDeadline-8   	  500000	      4013 ns/op	    1732 B/op	      23 allocs/op
--- BENCH: BenchmarkMemuWithDeadline-8
	memconn_test.go:239: serving memu:1522710151802014675
	memconn_test.go:239: serving memu:1522710151802462612
	memconn_test.go:239: serving memu:1522710151803324077
	memconn_test.go:239: serving memu:1522710151843170117
BenchmarkTCP-8                	   20000	     62645 ns/op	     902 B/op	      20 allocs/op
--- BENCH: BenchmarkTCP-8
	memconn_test.go:239: serving tcp:127.0.0.1:61231
	memconn_test.go:239: serving tcp:127.0.0.1:61233
	memconn_test.go:239: serving tcp:127.0.0.1:61334
	memconn_test.go:239: serving tcp:127.0.0.1:54960
BenchmarkTCPWithDeadline-8    	   20000	     76511 ns/op	     903 B/op	      20 allocs/op
--- BENCH: BenchmarkTCPWithDeadline-8
	memconn_test.go:239: serving tcp:127.0.0.1:58606
	memconn_test.go:239: serving tcp:127.0.0.1:58608
	memconn_test.go:239: serving tcp:127.0.0.1:58709
	memconn_test.go:239: serving tcp:127.0.0.1:52351
BenchmarkUNIX-8               	  100000	     24582 ns/op	    1424 B/op	      20 allocs/op
--- BENCH: BenchmarkUNIX-8
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/898168290.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/679687641.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/108764516.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/642938739.sock
BenchmarkUNIXWithDeadline-8   	  100000	     24080 ns/op	    1425 B/op	      20 allocs/op
--- BENCH: BenchmarkUNIXWithDeadline-8
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/401393206.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/006622237.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/006865112.sock
	memconn_test.go:239: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/465015895.sock
```

MemConn is faster and allocates fewer objects than the TCP and UNIX domain
sockets. While MemConn does allocate more memory, this is to be expected
since MemConn is an in-memory implementation of the `net.Conn` interface.
