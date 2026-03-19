# restaurant Microservice

## Quick Start

```bash
make build
make docker-build
make docker-up
```

## Endpoints

 - GET /api/restaurants - Search restaurants
 - GET /api/restaurants/{restaurantId} - Get restaurant details
 - GET /api/restaurants/{restaurantId}/similar - Find similar restaurants
 - GET /api/restaurants/compare - Compare restaurants

## Development

```bash
go run cmd/main.go
make test
make lint
```
