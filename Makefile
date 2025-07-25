# Makefile for Go project

# Variables
BINARY_NAME = persysctl
BINARY_DIR = bin
BINARY_PATH = $(BINARY_DIR)/$(BINARY_NAME)
GO = go
GOFLAGS = -v

# Default target
.PHONY: all
all: build

# Ensure bin directory exists
$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Build the binary into bin directory
.PHONY: build
build: $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY_PATH) main.go

# Run the application from bin directory
.PHONY: run
run: build
	./$(BINARY_PATH)

# Test the code (if you add tests later)
.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

# Clean up generated files
.PHONY: clean
clean:
	$(GO) clean
	rm -rf $(BINARY_DIR)

# Format the code
.PHONY: fmt
fmt:
	$(GO) fmt ./...

# Vet the code for potential issues
.PHONY: vet
vet:
	$(GO) vet ./...

# Update dependencies
.PHONY: deps
deps:
	$(GO) mod tidy
	$(GO) mod download

# Build and run with a single command from bin directory
.PHONY: dev
dev: build
	./$(BINARY_PATH)

# Check for linting issues (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Install the binary to $GOPATH/bin
.PHONY: install
install:
	$(GO) install $(GOFLAGS)

# Clean up client certificates
.PHONY: clean-cert clean-cert-client clean-cert-ca
clean-cert: clean-cert-client clean-cert-ca

clean-cert-client:
	rm -f ~/.persys/client.crt ~/.persys/client.key

clean-cert-ca:
	rm -f ~/.persys/ca.pem

# Help command to display available targets
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all      	   - Build the project into bin/ (default)"
	@echo "  build    	   - Build the binary into bin/"
	@echo "  run           - Build and run the application from bin/"
	@echo "  test          - Run tests"
	@echo "  clean         - Remove generated files and bin/ directory"
	@echo "  fmt           - Format the code"
	@echo "  vet           - Vet the code"
	@echo "  deps          - Update and download dependencies"
	@echo "  dev           - Build and run from bin/ for development"
	@echo "  lint          - Run linter (requires golangci-lint)"
	@echo "  install       - Install the binary to $$GOPATH/bin"
	@echo "  help          - Show this help message"