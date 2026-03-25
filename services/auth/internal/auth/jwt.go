package auth

import (
	"context"
	"fmt"
	"time"

	"MrFood/services/auth/config"

	"github.com/golang-jwt/jwt/v5"
)

type Auth struct {
	jwtSecret    []byte
	expiresHours int
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func New(ctx context.Context) (*Auth, error) {
	cfg := config.Get(ctx)
	if len(cfg.JWT.Secret) < 32 {
		return nil, fmt.Errorf("JWT secret too short: %d chars", len(cfg.JWT.Secret))
	}

	return &Auth{
		jwtSecret:    []byte(cfg.JWT.Secret),
		expiresHours: cfg.JWT.ExpiresHours,
	}, nil
}

func (a *Auth) CreateToken(userID, username string) (string, error) {
	cfg := config.Get(context.Background())
	expiresAt := time.Now().Add(time.Duration(cfg.JWT.ExpiresHours) * time.Hour)

	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "my-client-issuer",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

func (a *Auth) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return "", err
	}

	claims := token.Claims.(*Claims)
	return claims.UserID, nil
}
