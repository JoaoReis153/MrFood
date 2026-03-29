package repository

import (
	"MrFood/services/auth/internal/auth"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

const (
	blacklistKeyPrefix    = "blacklist:"
	tokenVersionKeyPrefix = "token_version:"
	tokenVersionTTL       = 24 * time.Hour * 30
)

func (r *Repository) StoreRefreshToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (token_id, user_id, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`
	_, err := r.DB.Exec(ctx, query, tokenID, userID, expiresAt)
	if err != nil {
		slog.Error("failed to store refresh token", "error", err)
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenID string) (*auth.RefreshTokenData, error) {
	query := `
		SELECT token_id, user_id, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token_id = $1
	`

	var t auth.RefreshTokenData
	err := r.DB.QueryRow(ctx, query, tokenID).Scan(
		&t.TokenID,
		&t.UserID,
		&t.ExpiresAt,
		&t.Revoked,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		slog.Error("failed to get refresh token", "error", err)
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &t, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	slog.Debug("revoking refresh token", "token_id", tokenID)
	query := `UPDATE refresh_tokens SET revoked = true WHERE token_id = $1`
	_, err := r.DB.Exec(ctx, query, tokenID)
	if err != nil {
		slog.Error("failed to revoke refresh token", "error", err)
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	slog.Debug("revoking all user tokens", "user_id", userID)
	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`

	_, err := r.DB.Exec(ctx, query, userID)
	if err != nil {
		slog.Error("failed to revoke all user tokens", "error", err)
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}

	return nil
}

func (r *Repository) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	key := blacklistKeyPrefix + tokenID
	err := r.Redis.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		slog.Error("failed to blacklist token", "error", err)
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	return nil
}

func (r *Repository) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := blacklistKeyPrefix + tokenID
	val, err := r.Redis.Exists(ctx, key).Result()
	if err != nil {
		slog.Error("failed to check token blacklist", "error", err)
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}
	return val > 0, nil
}

func (r *Repository) GetUserTokenVersion(ctx context.Context, userID string) (int, error) {
	key := tokenVersionKeyPrefix + userID

	val, err := r.Redis.Get(ctx, key).Int()
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, redis.Nil) {
		slog.Error("failed to get token version from cache", "error", err)
		return 0, fmt.Errorf("failed to get token version from cache: %w", err)
	}

	// cache miss - get from postgres
	version, err := r.getTokenVersionFromDB(ctx, userID)
	if err != nil {
		return 0, err
	}

	err = r.Redis.Set(ctx, key, version, tokenVersionTTL).Err()
	if err != nil {
		slog.Error("failed to set token version in redis", "error", err)
		return 0, fmt.Errorf("failed to set token version in redis: %w", err)
	}

	return version, nil
}

func (r *Repository) IncrementUserTokenVersion(ctx context.Context, userID string) (int64, error) {
	query := `
		INSERT INTO user_token_versions (user_id, version)
		VALUES ($1, 1)
		ON CONFLICT (user_id) DO UPDATE
		SET version = user_token_versions.version + 1
		RETURNING version
	`

	var newVersion int64
	err := r.DB.QueryRow(ctx, query, userID).Scan(&newVersion)
	if err != nil {
		slog.Error("failed to increment token version", "error", err)
		return 0, fmt.Errorf("failed to increment token version: %w", err)
	}

	key := tokenVersionKeyPrefix + userID
	_ = r.Redis.Set(ctx, key, newVersion, tokenVersionTTL).Err()

	return newVersion, nil
}

func (r *Repository) getTokenVersionFromDB(ctx context.Context, userID string) (int, error) {
	query := `SELECT version FROM user_token_versions WHERE user_id = $1`

	var version int
	err := r.DB.QueryRow(ctx, query, userID).Scan(&version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		slog.Error("failed to get token version from db", "error", err)
		return 0, fmt.Errorf("failed to get token version from db: %w", err)
	}

	return version, nil
}
