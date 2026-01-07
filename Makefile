.PHONY: build test clean install dev

# Build the binary
build:
	go build -o coffer .

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f coffer coverage.out coverage.html

# Install to GOPATH/bin
install:
	go install .

# Development: build and run
dev: build
	./coffer

# Tidy dependencies
tidy:
	go mod tidy
