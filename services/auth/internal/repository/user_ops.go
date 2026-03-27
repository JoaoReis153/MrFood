package repository

import (
	models "MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log/slog"
)

func (r *Repository) CreateUser(ctx context.Context, username, password, email string) (int64, string, error) {
	if ctx.Err() != nil {
		return 0, "", ctx.Err()
	}

	query := `
		INSERT INTO app_user (username, password, email)
		VALUES ($1, $2, $3)
		RETURNING user_id, username
	`

	var userId int64
	var returnedUsername string

	err := r.DB.QueryRow(ctx, query, username, password, email).Scan(&userId, &returnedUsername)
	if err != nil {
		slog.Error("failed to create user", "error", err)
		return 0, "", fmt.Errorf("failed to create user: %w", err)
	}

	return userId, returnedUsername, nil
}

func (r *Repository) GetUser(ctx context.Context, email string) (*models.User, error) {
	slog.Debug("getting user by email", "email", email)
	query := `
		SELECT user_id, username, password
		FROM app_user
		WHERE email = $1
	`

	var userId int64
	var username string
	var password string

	err := r.DB.QueryRow(ctx, query, email).Scan(&userId, &username, &password)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &models.User{
		ID:       userId,
		Username: username,
		Email:    email,
		Password: password,
	}, nil
}
