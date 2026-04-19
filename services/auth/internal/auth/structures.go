package auth

import (
	"MrFood/services/auth/config"
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	config     *config.JWTConfig
	tokenStore TokenStore
}

type TokenStore interface {
	StoreRefreshToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenID string) (*RefreshTokenData, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
	IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error)
	BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error
	GetUserTokenVersion(ctx context.Context, userID string) (int, error)
	IncrementUserTokenVersion(ctx context.Context, userID string) (int64, error)
}

type RefreshTokenData struct {
	TokenID   string
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func NewJWTService(config *config.JWTConfig, store TokenStore) *JWTService {
	return &JWTService{
		config:     config,
		tokenStore: store,
	}
}
