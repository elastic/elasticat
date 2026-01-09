.PHONY: build install clean logs test fmt fmt-check license-check license-add notice dist dist-platform dist-clean prep sloc release demo

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

# Check license headers (for CI) - fails if any Go files are missing headers
license-check:
	@./scripts/check-license.sh

# Add license headers to all Go files that need them
license-add:
	@./scripts/add-license.sh

# Generate a test log (for development)
test-log:
	@echo '{"timestamp":"$(shell date -Iseconds)","level":"INFO","message":"Test log from Makefile","service":"test-service"}' | \
		curl -X POST -H "Content-Type: application/json" -d @- http://localhost:4318/v1/logs || true

# Generate the NOTICE.txt file with third-party license information
notice:
	@echo "Generating NOTICE.txt"
	go mod tidy
	go mod download
	go list -m -json all | go run go.elastic.co/go-licence-detector \
		-includeIndirect \
		-rules scripts/notice/rules.json \
		-overrides scripts/notice/overrides.json \
		-noticeTemplate scripts/notice/NOTICE.txt.tmpl \
		-noticeOut NOTICE.txt \
		-depsOut ""

# Distribution variables
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build a distribution archive with binary + license files
dist: build
	@echo "Creating distribution archive..."
	@mkdir -p $(DIST_DIR)
	@cp bin/elasticat $(DIST_DIR)/
	@cp LICENSE.txt $(DIST_DIR)/
	@cp NOTICE.txt $(DIST_DIR)/
	@cp README.md $(DIST_DIR)/
	@echo "Distribution files ready in $(DIST_DIR)/"

# Cross-compile and package for a specific platform (used by CI)
# Usage: make dist-platform GOOS=linux GOARCH=amd64
dist-platform:
	@echo "Building for $(GOOS)/$(GOARCH)..."
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	$(eval ARCHIVE_EXT := $(if $(filter windows,$(GOOS)),.zip,.tar.gz))
	$(eval BINARY := elasticat-$(GOOS)-$(GOARCH)$(EXT))
	$(eval ARCHIVE_DIR := elasticat-$(GOOS)-$(GOARCH))
	$(eval ARCHIVE := elasticat-$(VERSION)-$(GOOS)-$(GOARCH)$(ARCHIVE_EXT))
	@mkdir -p $(DIST_DIR)/$(ARCHIVE_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(DIST_DIR)/$(ARCHIVE_DIR)/$(BINARY) ./cmd/elasticat
	@cp LICENSE.txt NOTICE.txt README.md $(DIST_DIR)/$(ARCHIVE_DIR)/
	@cd $(DIST_DIR) && \
		if [ "$(GOOS)" = "windows" ]; then \
			zip -r $(ARCHIVE) $(ARCHIVE_DIR); \
		else \
			tar -czvf $(ARCHIVE) $(ARCHIVE_DIR); \
		fi
	@rm -rf $(DIST_DIR)/$(ARCHIVE_DIR)
	@echo "Created $(DIST_DIR)/$(ARCHIVE)"

# Clean distribution artifacts
dist-clean:
	rm -rf $(DIST_DIR)

# Prepare code for PR (format, add license headers, update NOTICE)
prep: fmt license-add notice
	@echo "Code is ready for PR!"

# Create and push a release tag (runs validation first)
# Usage: make release VERSION=v1.0.0
release:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=v1.0.0"; exit 1; fi
	@if [ ! -f "release-notes/$(VERSION).md" ]; then \
		echo "ERROR: Missing release-notes/$(VERSION).md"; \
		echo "Create this file with release notes before running make release"; \
		exit 1; \
	fi
	@echo "Preparing release $(VERSION)..."
	@$(MAKE) test
	@$(MAKE) fmt-check
	@$(MAKE) license-check
	@echo "All checks passed. Creating tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "Release $(VERSION) pushed! GitHub Actions will create the release."

# Count source lines of code (excluding examples/stock-tracker)
sloc:
	@echo "=== Source Lines of Code ==="
	@echo ""
	@echo "Production code:"
	@find . -name '*.go' -not -name '*_test.go' -not -path './examples/stock-tracker/*' | xargs wc -l | tail -1 | awk '{print "  " $$1 " lines"}'
	@echo ""
	@echo "Test code:"
	@find . -name '*_test.go' -not -path './examples/stock-tracker/*' | xargs wc -l | tail -1 | awk '{print "  " $$1 " lines"}'
	@echo ""
	@echo "Total:"
	@find . -name '*.go' -not -path './examples/stock-tracker/*' | xargs wc -l | tail -1 | awk '{print "  " $$1 " lines"}'

# Generate demo GIF using VHS (requires: brew install vhs ttyd ffmpeg)
# Make sure elasticat is built and the stack is running with data first
demo: build
	@echo "Generating demo GIF..."
	@echo "Note: Stack must be running with data (elasticat up + some telemetry)"
	vhs demo.tape
	@echo "Demo GIF saved to docs/demo.gif"
