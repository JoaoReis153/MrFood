# restaurant Microservice

## Quick Start

```bash
make build
make docker-build
make docker-up
```

## Endpoints

- GET /api/v1/health - Health check

- GET /api/v1/ping - Ping endpoint

## Development

```bash
go run cmd/main.go
make test
make lint
```

## gRPC Implementation
- gRPC implementation consists of a single .proto file that can be found in "test_grpc/internal/api/grpc/proto". It defines the contracts between microservices (denoted by the "rpc" keyword) and the messages sent by each of them.

- Whenever the .proto file is changed, navigate to "MrFood/services/<service_name>" and execute the following command:

`make proto`

- This command generates the pb/ directory that contains 2 .pb.go files, needed for gRPC to work.

- **Important note**: protoc needs to installed in order to run the scripts. Install it via:
    - Linux
    `sudo dnf install protobuf-compiler`

    - MacOS
    `brew install protobuf`
