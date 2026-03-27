package auth

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidClaims    = errors.New("invalid token claims")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrTokenRevoked     = errors.New("token has been revoked")
)
