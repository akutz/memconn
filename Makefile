all: build

build: memconn.a
memconn.a: $(filter-out %_test.go, $(wildcard *.go))
	go build -o $@

benchmark:
	go test -bench . -run Bench -benchmem .

test:
	go test
	go test -race -run 'Race$$'
