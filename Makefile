BINARY=kubectl-pvbench
PACKAGE=./cmd/kubectl-pvbench

.PHONY: build test clean

build:
	go build -o bin/$(BINARY) $(PACKAGE)

test:
	go test ./...

clean:
	rm -rf bin/