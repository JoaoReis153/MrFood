.PHONY: build test lint docker-build docker-up docker-down clean proto

# Config
SERVICE_NAME := notification
BINARY := bin/server
PROTO_PATH := internal/api/grpc/

build:
	go build -o $(BINARY) ./cmd

test:
	go test -v -race ./...

lint:
	golangci-lint run

docker-build:
	docker build -t $(SERVICE_NAME):latest .

docker-up:
	docker run -d -p 50051:50051 \
		--name $(SERVICE_NAME) \
		-e APP_SERVER_HOST=0.0.0.0 \
		-e APP_SERVER_PORT=50051 \
		-e APP_SERVER_TIMEOUT=30s \
		-e APP_LOG_LEVEL=info \
		$(SERVICE_NAME):latest

docker-down:
	docker stop $(SERVICE_NAME) 2>/dev/null || true
	docker rm $(SERVICE_NAME) 2>/dev/null || true

clean:
	rm -rf $(BINARY)

proto:
	protoc --go_out=$(PROTO_PATH) --go-grpc_out=$(PROTO_PATH) $(PROTO_PATH)/proto/protofile.proto
