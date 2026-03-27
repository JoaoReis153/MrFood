package auth

import (
	"MrFood/services/auth/config"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type mockTokenStore struct {
	storeRefreshTokenFn       func(ctx context.Context, tokenID, userID string, expiresAt time.Time) error
	getRefreshTokenFn         func(ctx context.Context, tokenID string) (*RefreshTokenData, error)
	revokeRefreshTokenFn      func(ctx context.Context, tokenID string) error
	revokeAllUserTokensFn     func(ctx context.Context, userID string) error
	isTokenBlacklistedFn      func(ctx context.Context, tokenID string) (bool, error)
	blacklistTokenFn          func(ctx context.Context, tokenID string, expiresAt time.Time) error
	getUserTokenVersionFn     func(ctx context.Context, userID string) (int, error)
	incrementUserTokenVersion func(ctx context.Context, userID string) (int64, error)
}

func (m *mockTokenStore) StoreRefreshToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
	if m.storeRefreshTokenFn == nil {
		return nil
	}
	return m.storeRefreshTokenFn(ctx, tokenID, userID, expiresAt)
}

func (m *mockTokenStore) GetRefreshToken(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
	if m.getRefreshTokenFn == nil {
		return nil, nil
	}
	return m.getRefreshTokenFn(ctx, tokenID)
}

func (m *mockTokenStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	if m.revokeRefreshTokenFn == nil {
		return nil
	}
	return m.revokeRefreshTokenFn(ctx, tokenID)
}

func (m *mockTokenStore) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if m.revokeAllUserTokensFn == nil {
		return nil
	}
	return m.revokeAllUserTokensFn(ctx, userID)
}

func (m *mockTokenStore) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	if m.isTokenBlacklistedFn == nil {
		return false, nil
	}
	return m.isTokenBlacklistedFn(ctx, tokenID)
}

func (m *mockTokenStore) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	if m.blacklistTokenFn == nil {
		return nil
	}
	return m.blacklistTokenFn(ctx, tokenID, expiresAt)
}

func (m *mockTokenStore) GetUserTokenVersion(ctx context.Context, userID string) (int, error) {
	if m.getUserTokenVersionFn == nil {
		return 0, nil
	}
	return m.getUserTokenVersionFn(ctx, userID)
}

func (m *mockTokenStore) IncrementUserTokenVersion(ctx context.Context, userID string) (int64, error) {
	if m.incrementUserTokenVersion == nil {
		return 0, nil
	}
	return m.incrementUserTokenVersion(ctx, userID)
}

func newTestJWTService(store TokenStore) *JWTService {
	return NewJWTService(&config.JWTConfig{
		AccessTokenSecret:  strings.Repeat("a", 40),
		RefreshTokenSecret: strings.Repeat("r", 40),
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    24 * time.Hour,
	}, store)
}

func tokenString(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestGenerateAccessToken(t *testing.T) {
	t.Run("error on token version lookup", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 0, errors.New("db down")
			},
		})

		_, err := svc.GenerateAccessToken(context.Background(), "u1", "john")
		if err == nil || !strings.Contains(err.Error(), "failed to get token version") {
			t.Fatalf("expected version error, got: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 3, nil
			},
		})

		tok, err := svc.GenerateAccessToken(context.Background(), "u1", "john")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claims, err := svc.ValidateAccessToken(context.Background(), tok)
		if err != nil {
			t.Fatalf("validate generated token: %v", err)
		}
		if claims.UserID != "u1" || claims.Username != "john" || claims.TokenType != "access" {
			t.Fatalf("unexpected claims: %+v", claims)
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	t.Run("error on storing token", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			storeRefreshTokenFn: func(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
				return errors.New("redis down")
			},
		})

		_, err := svc.GenerateRefreshToken(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to store refresh token") {
			t.Fatalf("expected store error, got: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		called := false
		svc := newTestJWTService(&mockTokenStore{
			storeRefreshTokenFn: func(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
				called = true
				if tokenID == "" || userID != "u1" || expiresAt.IsZero() {
					t.Fatalf("unexpected store args tokenID=%q userID=%q expiresAt=%v", tokenID, userID, expiresAt)
				}
				return nil
			},
		})

		tok, err := svc.GenerateRefreshToken(context.Background(), "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok == "" {
			t.Fatal("expected non-empty refresh token")
		}
		if !called {
			t.Fatal("expected StoreRefreshToken to be called")
		}
	})
}

func TestValidateAccessToken(t *testing.T) {
	now := time.Now()
	baseClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "tid-1",
			Subject:   "u1",
			IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			Issuer:    "my-client",
		},
		UserID:       "u1",
		Username:     "john",
		TokenVersion: 2,
		TokenType:    "access",
	}

	t.Run("invalid token string", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.ValidateAccessToken(context.Background(), "invalid-token")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got: %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		expiredClaims := baseClaims
		expiredClaims.ExpiresAt = jwt.NewNumericDate(now.Add(-1 * time.Minute))
		tok := tokenString(t, strings.Repeat("a", 40), expiredClaims)
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if !errors.Is(err, ErrExpiredToken) {
			t.Fatalf("expected ErrExpiredToken, got: %v", err)
		}
	})

	t.Run("invalid token type", func(t *testing.T) {
		wrongTypeClaims := baseClaims
		wrongTypeClaims.TokenType = "refresh"
		tok := tokenString(t, strings.Repeat("a", 40), wrongTypeClaims)
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if !errors.Is(err, ErrInvalidTokenType) {
			t.Fatalf("expected ErrInvalidTokenType, got: %v", err)
		}
	})

	t.Run("blacklist check error", func(t *testing.T) {
		tok := tokenString(t, strings.Repeat("a", 40), baseClaims)
		svc := newTestJWTService(&mockTokenStore{
			isTokenBlacklistedFn: func(ctx context.Context, tokenID string) (bool, error) {
				return false, errors.New("redis error")
			},
		})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if err == nil || !strings.Contains(err.Error(), "failed to check token blacklist") {
			t.Fatalf("expected blacklist check error, got: %v", err)
		}
	})

	t.Run("blacklisted token", func(t *testing.T) {
		tok := tokenString(t, strings.Repeat("a", 40), baseClaims)
		svc := newTestJWTService(&mockTokenStore{
			isTokenBlacklistedFn: func(ctx context.Context, tokenID string) (bool, error) {
				return true, nil
			},
		})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if !errors.Is(err, ErrTokenRevoked) {
			t.Fatalf("expected ErrTokenRevoked, got: %v", err)
		}
	})

	t.Run("token version lookup error", func(t *testing.T) {
		tok := tokenString(t, strings.Repeat("a", 40), baseClaims)
		svc := newTestJWTService(&mockTokenStore{
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 0, errors.New("db error")
			},
		})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if err == nil || !strings.Contains(err.Error(), "failed to get user token version") {
			t.Fatalf("expected token version error, got: %v", err)
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		tok := tokenString(t, strings.Repeat("a", 40), baseClaims)
		svc := newTestJWTService(&mockTokenStore{
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 3, nil
			},
		})
		_, err := svc.ValidateAccessToken(context.Background(), tok)
		if !errors.Is(err, ErrTokenRevoked) {
			t.Fatalf("expected ErrTokenRevoked, got: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		tok := tokenString(t, strings.Repeat("a", 40), baseClaims)
		svc := newTestJWTService(&mockTokenStore{
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 2, nil
			},
		})
		claims, err := svc.ValidateAccessToken(context.Background(), tok)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.UserID != "u1" || claims.TokenVersion != 2 {
			t.Fatalf("unexpected claims: %+v", claims)
		}
	})
}

func TestRevokeAllUserTokens(t *testing.T) {
	t.Run("revoke fails", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			revokeAllUserTokensFn: func(ctx context.Context, userID string) error {
				return errors.New("db error")
			},
		})
		err := svc.RevokeAllUserTokens(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to revoke all user tokens") {
			t.Fatalf("expected revoke error, got: %v", err)
		}
	})

	t.Run("increment fails", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			incrementUserTokenVersion: func(ctx context.Context, userID string) (int64, error) {
				return 0, errors.New("db error")
			},
		})
		err := svc.RevokeAllUserTokens(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to increment token version") {
			t.Fatalf("expected increment error, got: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			incrementUserTokenVersion: func(ctx context.Context, userID string) (int64, error) {
				return 2, nil
			},
		})
		if err := svc.RevokeAllUserTokens(context.Background(), "u1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRefreshTokens(t *testing.T) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "refresh-1",
			Subject:   "u1",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			Issuer:    "my-client",
		},
		UserID:    "u1",
		Username:  "john",
		TokenType: "refresh",
	}

	t.Run("parse fails", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.RefreshTokens(context.Background(), "invalid-token")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got: %v", err)
		}
	})

	t.Run("wrong token type", func(t *testing.T) {
		wrong := claims
		wrong.TokenType = "access"
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), wrong))
		if !errors.Is(err, ErrInvalidTokenType) {
			t.Fatalf("expected ErrInvalidTokenType, got: %v", err)
		}
	})

	t.Run("state lookup error", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getRefreshTokenFn: func(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
				return nil, errors.New("db error")
			},
		})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if err == nil || !strings.Contains(err.Error(), "failed to get stored refresh token") {
			t.Fatalf("expected state lookup error, got: %v", err)
		}
	})

	t.Run("token missing", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got: %v", err)
		}
	})

	t.Run("token revoked", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getRefreshTokenFn: func(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
				return &RefreshTokenData{TokenID: tokenID, UserID: "u1", Revoked: true}, nil
			},
		})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if !errors.Is(err, ErrTokenRevoked) {
			t.Fatalf("expected ErrTokenRevoked, got: %v", err)
		}
	})

	t.Run("rotation fails", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getRefreshTokenFn: func(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
				return &RefreshTokenData{TokenID: tokenID, UserID: "u1", Revoked: false}, nil
			},
			revokeRefreshTokenFn: func(ctx context.Context, tokenID string) error {
				return errors.New("rotate fail")
			},
		})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if err == nil || !strings.Contains(err.Error(), "failed to revoke old refresh token") {
			t.Fatalf("expected rotate error, got: %v", err)
		}
	})

	t.Run("generation fails", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getRefreshTokenFn: func(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
				return &RefreshTokenData{TokenID: tokenID, UserID: "u1", Revoked: false}, nil
			},
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 0, errors.New("version fail")
			},
		})
		_, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if err == nil || !strings.Contains(err.Error(), "failed to get token version") {
			t.Fatalf("expected generation error, got: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := newTestJWTService(&mockTokenStore{
			getRefreshTokenFn: func(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
				return &RefreshTokenData{TokenID: tokenID, UserID: "u1", Revoked: false}, nil
			},
			getUserTokenVersionFn: func(ctx context.Context, userID string) (int, error) {
				return 5, nil
			},
		})
		pair, err := svc.RefreshTokens(context.Background(), tokenString(t, strings.Repeat("r", 40), claims))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pair.AccessToken == "" || pair.RefreshToken == "" {
			t.Fatalf("expected token pair, got: %+v", pair)
		}
	})
}
