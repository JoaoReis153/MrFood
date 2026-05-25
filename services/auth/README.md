# Auth Service

## Overview

- Transport: **gRPC only**.
- API contract: `internal/api/grpc/proto/protofile.proto`.
- Storage: **Keycloak only** — no database, no Redis. All user storage, credential validation, and session management is delegated to Keycloak.
- Server startup: `cmd/main.go` initializes config, Keycloak client, and runs the gRPC server.
- Shutdown: graceful stop on `SIGINT`/`SIGTERM`.

## Operations (gRPC)

Service: `AuthService`

- `PingPong(Ping) -> Pong`
- `RegisterProcess(Register) -> RegisterResponse`
- `LoginProcess(Login) -> LoginResponse`
- `RefreshTokenProcess(RefreshRequest) -> RefreshResponse`
- `LogoutProcess(LogoutRequest) -> LogoutResponse`

## Workflow and Logic

### 1) Register

1. Receives `username`, `email`, `password`.
2. Creates the user in Keycloak via the Admin API.
3. Sends a registration email via the notification service.
4. Returns `id` (FNV hash of the Keycloak UUID) and `username`.

### 2) Login

1. Calls Keycloak's token endpoint with the provided email and password.
2. Parses `sub`, `preferred_username`, and `email` from the returned Keycloak JWT.
3. Mints a short-lived app JWT (`HS256`) containing `user_id`, `username`, and `email`.
4. Returns the app access token, the Keycloak refresh token, and basic user info.

### 3) Refresh Token

1. Passes the refresh token to Keycloak's refresh endpoint.
2. Parses the new Keycloak JWT for `sub`, `preferred_username`, and `email`.
3. Mints a new app JWT and returns it alongside the new Keycloak refresh token.

### 4) Logout

1. Parses `sub` from the app JWT.
2. Calls Keycloak's Admin API to revoke all sessions for that user.

## Token Format

App access tokens are signed HS256 JWTs with the following claims:

| Claim      | Description                             |
|------------|-----------------------------------------|
| `sub`      | Keycloak user UUID                      |
| `user_id`  | Same as `sub`                           |
| `username` | Keycloak `preferred_username`           |
| `email`    | Keycloak `email`                        |
| `iss`      | `mrfood-auth`                           |
| `iat`/`exp`| Issued at / expires at (TTL: 5 minutes) |

Refresh tokens are Keycloak-issued JWTs passed through as-is.

## Configuration

Main environment variables:

- Server: `APP_SERVER_PORT`, `APP_LOG_LEVEL`
- Keycloak: `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT_ID`, `KEYCLOAK_CLIENT_SECRET`, `KEYCLOAK_ADMIN_USER`, `KEYCLOAK_ADMIN_PASS`
- JWT: `APP_JWT_SECRET`
- Notification: `NOTIFICATION_GRPC_ADDR`

Defaults and validation are defined in `config/config.go`.

## Development

### Run locally

```bash
go run ./cmd/main.go
```

### Build and test

```bash
make build
make test
```

### Run with Docker Compose (from `services/`)

```bash
docker compose up --build auth keycloak
```

## gRPC Code Generation

Proto file: `internal/api/grpc/proto/protofile.proto`

Generate stubs (from `services/auth`):

```bash
make proto
```

- macOS: `brew install protobuf`
- Linux (dnf): `sudo dnf install protobuf-compiler`
