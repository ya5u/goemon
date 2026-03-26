BINARY_NAME=goemon
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build run test clean deploy-raspi

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/goemon/

run:
	go run ./cmd/goemon/ chat

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/goemon/

deploy:
	@test -n "$(TARGET)" || (echo "Usage: make deploy TARGET=user@host:/path"; exit 1)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/goemon/
	scp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(TARGET)
