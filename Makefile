IMAGE_NAME ?= $(DOCKER_HUB_USER)/shelly-plugs-exporter:latest

.PHONY: all build test docker-build docker-push lint

all: test build

build:
	go build -o bin/exporter ./cmd/exporter/

test:
	go test ./...

docker-build:
	docker build -t $(IMAGE_NAME) .

docker-push:
	docker push $(IMAGE_NAME)

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"
