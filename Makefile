.PHONY: build install clean up down logs test fmt fmt-check

# Build the elasticat binary
build:
	go build -o bin/elasticat ./cmd/elasticat

# Install to GOPATH/bin
install:
	go install ./cmd/elasticat

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
	./bin/elasticat logs

# Tail logs
tail: build
	./bin/elasticat tail

# Check stack status
status: build
	./bin/elasticat status

# Run tests
test:
	go test ./...

# Format code
fmt:
	gofmt -s -w .

# Check formatting (for CI) - fails if files need formatting
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

# Lint code
lint:
	golangci-lint run

# Generate a test log (for development)
test-log:
	@echo '{"timestamp":"$(shell date -Iseconds)","level":"INFO","message":"Test log from Makefile","service":"test-service"}' | \
		curl -X POST -H "Content-Type: application/json" -d @- http://localhost:4318/v1/logs || true
