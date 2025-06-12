.PHONY: build run clean test

build:
	go build -o bin/mcp-lsp-bridge .

run: build
	./bin/mcp-lsp-bridge

clean:
	rm -rf bin/

test:
	go test ./...

install-deps:
	go mod tidy
	go mod download
