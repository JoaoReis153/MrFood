# test_grpc Microservice

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
