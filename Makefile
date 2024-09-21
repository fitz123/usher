PROJECT_REPO := "fitz123/usher"
VERSION := $(shell git describe --tags --always --dirty="-dev")
BUILD_DATE := $(shell date +%F-%T)
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)
BINARY_NAME := usher

.PHONY: test bench
test:
	go test -race ./... -count=1

bench:
	go test -bench=. -run=^a ./...

.PHONY: buildDocker
buildDocker:
	mkdir -p bin
	@echo "Building the Docker image..."
	@docker build --build-arg VERSION="$(VERSION)" --build-arg BUILD_DATE="$(BUILD_DATE)" -t $(PROJECT_REPO):latest -f build/Dockerfile .
	@echo "Checking leftovers..."
	@docker ps -a -f name=temp-container | grep temp-container && docker rm temp-container || true
	@echo "Creating a temporary container to extract the binary..."
	@docker create --name temp-container $(PROJECT_REPO):latest
	@echo "Copying the binary from the Docker container..."
	@docker cp temp-container:/app/bin/$(BINARY_NAME) bin/$(BINARY_NAME)
	@echo "Cleaning up temporary container..."
	@docker rm temp-container

.PHONY: build
build:
	mkdir -p bin
	@echo "Building the application..."
	@CGO_ENABLED=0 go build -o bin/$(BINARY_NAME) main.go
