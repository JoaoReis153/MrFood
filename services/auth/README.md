# Auth Service

## Current Status

- Transport: **gRPC only** (no REST endpoints exposed by this service).
- API contract: `internal/api/grpc/proto/protofile.proto`.
- Server startup: `cmd/main.go` initializes config, logger, app dependencies, then runs the gRPC server.
- Dependencies: PostgreSQL (users, refresh tokens, token versions) + Redis (blacklist + token-version cache).
- Shutdown: graceful stop on `SIGINT`/`SIGTERM`, with connection cleanup.

Implemented and covered by unit tests:

- User registration and lookup service logic.
- Login with password verification + token pair generation.
- Refresh token validation + rotation.
- Logout with access-token blacklist and user-wide token revocation.

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
2. Hashes password with bcrypt.
3. Validates user payload (`username >= 3`, valid email, password required).
4. Stores user in `app_user`.
5. Returns only `id` and `username`.

### 2) Login

1. Fetches user by email.
2. Compares provided password against stored bcrypt hash.
3. Revokes all previous user refresh tokens.
4. Increments user token version (invalidates older access tokens).
5. Generates new token pair:
   - Access token: JWT (`HS256`) with `user_id`, `username`, `token_version`, `token_type=access`.
   - Refresh token: JWT (`HS256`) with `user_id`, `token_type=refresh`, persisted in `refresh_tokens`.
6. Returns access token, refresh token, and basic user info.

### 3) Refresh Token

1. Parses and verifies refresh JWT signature and expiry.
2. Ensures token type is `refresh`.
3. Checks refresh token state in DB (exists and not revoked).
4. Revokes old refresh token (rotation).
5. Issues and stores a new token pair.

### 4) Logout

1. Validates access token.
2. Blacklists the specific access token in Redis until its expiration.
3. Revokes all refresh tokens for that user in DB.
4. Increments token version to invalidate other access tokens.

## Token Revocation Strategy

The service uses a layered revocation model:

- **Access token blacklist** in Redis (`blacklist:<token_id>`).
- **Per-user token version** in DB + Redis cache (`token_version:<user_id>`).
- **Refresh token revocation** in PostgreSQL (`refresh_tokens.revoked`).

This allows immediate single-token revocation (logout), plus full-session invalidation (login/logout-all behavior).

## Data Model

- `app_user(user_id, username, password, email)`
- `refresh_tokens(token_id, user_id, expires_at, revoked, created_at)`
- `user_token_versions(user_id, version)`

Schema file: `db_setup.sql`.

## Configuration

Main environment variables used by the service:

- Server/logging: `APP_SERVER_HOST`, `APP_SERVER_PORT`, `APP_SERVER_TIMEOUT`, `APP_LOG_LEVEL`
- DB: `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS`, `DB_MIN_CONNS`, `DB_MAX_CONNS`, `DB_MAX_CONN_LIFETIME`, `DB_HEALTH_CHECK_PERIOD`
- Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASS`, `REDIS_DB`
- JWT: `APP_JWT_ACCESS_TOKEN_SECRET`, `APP_JWT_REFRESH_TOKEN_SECRET`

Defaults and validation rules are defined in `config/config.go`.

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
docker compose up --build auth auth_db redis
```

## gRPC Code Generation

Proto file:

- `internal/api/grpc/proto/protofile.proto`

Generate stubs (from `services/auth`):

```bash
make proto
```

This updates generated files under `internal/api/grpc/pb`.

`protoc` installation examples:

- macOS: `brew install protobuf`
- Linux (dnf): `sudo dnf install protobuf-compiler`
