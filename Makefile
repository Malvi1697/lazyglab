BINARY_NAME=lazyglab
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")

.PHONY: build run test clean install lint fmt vet check release-dry

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html

install:
	go install -ldflags "-s -w -X main.version=$(VERSION)" .

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w -local github.com/Malvi1697/lazyglab .

vet:
	go vet ./...

tidy:
	go mod tidy

# Run all checks (lint + test + vet)
check: vet lint test

# Test GoReleaser config locally
release-dry:
	goreleaser release --snapshot --clean
