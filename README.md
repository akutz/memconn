# MemConn
MemConn is an in-memory network connection for Go.

## Performance
The benchmark results illustrates MemConn's performance against TCP
and UNIX domain sockets:

```shell
$ go test -benchmem -bench Benchmark -run Benchmark -v
goos: darwin
goarch: amd64
pkg: github.com/akutz/memconn
BenchmarkMemConn-8   	 1000000	      1750 ns/op	    1385 B/op	      15 allocs/op
--- BENCH: BenchmarkMemConn-8
	memconn_test.go:105: serving memu:1522087455339984096
	memconn_test.go:105: serving memu:1522087455340519873
	memconn_test.go:105: serving memu:1522087455341366262
	memconn_test.go:105: serving memu:1522087455356749963
BenchmarkTCP-8       	   20000	     78873 ns/op	     903 B/op	      20 allocs/op
--- BENCH: BenchmarkTCP-8
	memconn_test.go:105: serving tcp:127.0.0.1:57013
	memconn_test.go:105: serving tcp:127.0.0.1:57015
	memconn_test.go:105: serving tcp:127.0.0.1:57116
	memconn_test.go:105: serving tcp:127.0.0.1:50746
BenchmarkUnix-8      	   50000	     38214 ns/op	    3837 B/op	      58 allocs/op
--- BENCH: BenchmarkUnix-8
	memconn_test.go:105: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/140928442.sock
	memconn_test.go:105: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/908553169.sock
	memconn_test.go:105: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/327038716.sock
	memconn_test.go:105: serving unix:/var/folders/48/52fjq9r10zx3gnm7j57th0500000gn/T/797689899.sock
PASS
ok  	github.com/akutz/memconn	6.276s
```

MemConn is faster and allocates fewer objects than the TCP and UNIX domain
sockets. While MemConn does allocate more memory, this is to be expected
since MemConn is an in-memory implementation of the `net.Conn` interface.
