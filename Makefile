BINARY_NAME=lazyglab
VERSION?=0.1.0-dev

.PHONY: build run test clean install lint

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME)

install:
	go install -ldflags "-X main.version=$(VERSION)" .

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
