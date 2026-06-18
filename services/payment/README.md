# Payment Service

Transport: gRPC only. API contract: `internal/api/grpc/proto/protofile.proto`.

```bash
go run ./cmd/main.go
make build && make test
make docker-build && make docker-up
make proto  # after proto changes — requires protoc
```
