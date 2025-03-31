# Binary name
BINARY_NAME=monitor
BINARY_PATH=cmd/monitor/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFLAGS=-v

# Main package path
MAIN_PACKAGE=./cmd/monitor

.PHONY: all build run test clean tidy

# Default target
all: clean run

# Build the binary
build:
	@echo "Building..."
	$(GOBUILD) $(GOFLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_PATH)"

# Run the application
run: build
	@echo "Running..."
	./$(BINARY_PATH)

# Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) ./... $(GOFLAGS)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_PATH)
	@echo "Clean complete"

# Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	$(GOMOD) tidy
	@echo "Tidy complete"

# Help target
help:
	@echo "Available targets:"
	@echo "  make build    - Build the binary"
	@echo "  make run     - Build and run the application"
	@echo "  make test    - Run all tests"
	@echo "  make clean   - Remove build artifacts"
	@echo "  make tidy    - Run go mod tidy"
	@echo "  make all     - Clean and build"
	@echo "  make help    - Show this help message" 