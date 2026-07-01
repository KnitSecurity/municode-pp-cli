.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/municode-pp-cli$(BIN_EXT) ./cmd/municode-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/municode-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/municode-pp-mcp$(BIN_EXT) ./cmd/municode-pp-mcp

install-mcp:
	go install ./cmd/municode-pp-mcp

build-all: build build-mcp
