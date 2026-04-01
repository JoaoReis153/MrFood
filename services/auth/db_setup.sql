CREATE TABLE app_user(
    user_id NUMERIC(22, 0) PRIMARY KEY,
     username VARCHAR(50) NOT NULL UNIQUE,
     password VARCHAR(60) NOT NULL,
     email VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE refresh_tokens (
    token_id   TEXT PRIMARY KEY,
    user_id    NUMERIC(22, 0) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_token_versions (
    user_id NUMERIC(22, 0) PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1
);