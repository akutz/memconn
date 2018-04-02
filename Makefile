all: build

build: memconn.a
memconn.a: $(filter-out %_test.go, $(wildcard *.go))
	go build -o $@

BENCH ?= .

benchmark:
	go test -bench $(BENCH) -run Bench -benchmem .

benchmark-mypipe:
	go test -tags mypipe -bench $(BENCH) -run Bench -benchmem .

test:
	go test
	go test -race -run 'Race$$'
