package keycloak

import "errors"

var (
	// ErrInvalidCredentials is returned when username/password are wrong.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUserAlreadyExists is returned when a user with that email/username already exists.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrUserNotFound is returned when no user matches the query.
	ErrUserNotFound = errors.New("user not found")

	// ErrTokenInactive is returned when token introspection reports the token is not active.
	ErrTokenInactive = errors.New("token is not active")
)
