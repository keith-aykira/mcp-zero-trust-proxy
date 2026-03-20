.PHONY: all build test lint clean docker-build docker-run

VERSION ?= dev
BINARY  := bin/mcpproxy
IMAGE   := mcpzerotrust/proxy

# Default: lint, test, then build
all: lint test build

## build: Compile the mcpproxy binary into bin/
build:
	@mkdir -p bin
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY) ./cmd/mcpproxy

## test: Run all tests with race detector
test:
	go test ./... -v -race -count=1

## lint: Run go vet for static analysis
lint:
	go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/

## docker-build: Build the Docker image
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):dev .

## docker-run: Run the proxy in Docker (requires config.yaml in current directory)
docker-run:
	docker run --rm -p 8080:8080 \
		-v ./config.yaml:/etc/mcpproxy/config.yaml:ro \
		$(IMAGE):dev

## docker-run-example: Run with the bundled example config (no upstream MCP server — for testing only)
docker-run-example:
	docker run --rm -p 8080:8080 $(IMAGE):dev

## version: Print the current version
version:
	@echo $(VERSION)
