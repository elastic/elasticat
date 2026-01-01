.PHONY: build install clean up down logs test

# Build the turbodevlog binary
build:
	go build -o bin/turbodevlog ./cmd/turbodevlog

# Install to GOPATH/bin
install:
	go install ./cmd/turbodevlog

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Start the Docker stack
up:
	cd docker && docker compose up -d

# Start with Kibana
up-kibana:
	cd docker && docker compose --profile kibana up -d

# Start with MCP server (for AI assistant integration)
up-mcp:
	cd docker && docker compose --profile mcp up -d

# Start with everything (Kibana + MCP)
up-all:
	cd docker && docker compose --profile kibana --profile mcp up -d

# Stop the Docker stack
down:
	cd docker && docker compose down

# Open the log viewer
logs: build
	./bin/turbodevlog logs

# Tail logs
tail: build
	./bin/turbodevlog tail

# Check stack status
status: build
	./bin/turbodevlog status

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate a test log (for development)
test-log:
	@echo '{"timestamp":"$(shell date -Iseconds)","level":"INFO","message":"Test log from Makefile","service":"test-service"}' | \
		curl -X POST -H "Content-Type: application/json" -d @- http://localhost:4318/v1/logs || true
