BINARY  := kubeOptimum
MODULE  := github.com/millicano106/kubeOptimum
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build test lint install clean

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./... -v

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) .

clean:
	rm -f $(BINARY)
