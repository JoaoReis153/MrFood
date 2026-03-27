package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func (s *JWTService) GenerateAccessToken(ctx context.Context, userID, username string) (string, error) {
	version, err := s.tokenStore.GetUserTokenVersion(ctx, userID)
	if err != nil {
		slog.Error("failed to get token version", "error", err)
		return "", fmt.Errorf("failed to get token version: %w", err)
	}

	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessTokenTTL)),
			Issuer:    "my-client",
		},
		UserID:       userID,
		Username:     username,
		TokenVersion: version,
		TokenType:    "access",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(s.config.AccessTokenSecret))
	if err != nil {
		slog.Error("failed to sign access token", "error", err)
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return signedToken, nil
}

func (s *JWTService) GenerateRefreshToken(ctx context.Context, userID string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.RefreshTokenTTL)
	tokenID := uuid.NewString()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "my-client",
		},
		UserID:    userID,
		TokenType: "refresh",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(s.config.RefreshTokenSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	if err := s.tokenStore.StoreRefreshToken(ctx, tokenID, userID, expiresAt); err != nil {
		return "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return signedToken, nil
}

func (s *JWTService) GenerateTokenPair(ctx context.Context, userID, username string) (*TokenPair, error) {
	slog.Debug("generating token pair", "user_id", userID, "username", username)
	accessToken, err := s.GenerateAccessToken(ctx, userID, username)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.config.AccessTokenTTL),
	}, nil
}

func (s *JWTService) ValidateAccessToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.config.AccessTokenSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			slog.Error("access token expired", "error", err)
			return nil, ErrExpiredToken
		}
		slog.Error("failed to parse access token", "error", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	if claims.TokenType != "access" {
		slog.Error("invalid token type", "toke_type", claims.TokenType)
		return nil, ErrInvalidTokenType
	}

	blacklisted, err := s.tokenStore.IsTokenBlacklisted(ctx, claims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check token blacklist: %w", err)
	}
	if blacklisted {
		return nil, ErrTokenRevoked
	}

	currentVersion, err := s.tokenStore.GetUserTokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user token version: %w", err)
	}
	if claims.TokenVersion < currentVersion {
		return nil, ErrTokenRevoked
	}

	return claims, nil
}

func (s *JWTService) RevokeAccessToken(ctx context.Context, tokenString string) error {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return ErrInvalidClaims
	}

	return s.tokenStore.BlacklistToken(ctx, claims.ID, claims.ExpiresAt.Time)
}

func (s *JWTService) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if err := s.tokenStore.RevokeAllUserTokens(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}

	_, err := s.tokenStore.IncrementUserTokenVersion(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to increment token version: %w", err)
	}

	return nil
}

func (s *JWTService) RefreshTokens(ctx context.Context, tokenStr string) (*TokenPair, error) {
	claims, err := s.parseRefreshToken(tokenStr)
	if err != nil {
		return nil, err
	}

	if err := s.validateRefreshTokenState(ctx, claims); err != nil {
		return nil, err
	}

	if err := s.rotateRefreshToken(ctx, claims); err != nil {
		return nil, err
	}

	return s.GenerateTokenPair(ctx, claims.UserID, claims.Username)
}

func (s *JWTService) parseRefreshToken(refreshTokenString string) (*Claims, error) {
	slog.Debug("parsing refresh token", "token", refreshTokenString)
	token, err := jwt.ParseWithClaims(
		refreshTokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.config.RefreshTokenSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.TokenType != "refresh" {
		return nil, ErrInvalidTokenType
	}

	slog.Debug("refresh token parsed successfully", "token", token, "claims", claims)
	return claims, nil
}

func (s *JWTService) validateRefreshTokenState(ctx context.Context, claims *Claims) error {
	slog.Debug("validating refresh token state", "user_id", claims.UserID, "token_id", claims.ID)
	storedToken, err := s.tokenStore.GetRefreshToken(ctx, claims.ID)
	if err != nil {
		return fmt.Errorf("failed to get stored refresh token: %w", err)
	}
	if storedToken == nil {
		return ErrInvalidToken
	}

	if storedToken.Revoked {
		return ErrTokenRevoked
	}

	return nil
}

func (s *JWTService) rotateRefreshToken(ctx context.Context, claims *Claims) error {
	slog.Debug("rotating refresh token", "user_id", claims.UserID, "token_id", claims.ID)
	if err := s.tokenStore.RevokeRefreshToken(ctx, claims.ID); err != nil {
		return fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	return nil
}
