package service

import (
	"MrFood/services/auth/internal/repository"
	"MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log/slog"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) StoreUser(ctx context.Context, user *pkg.User) (*pkg.User, error) {
	if err := pkg.ValidateUser(*user); err != nil {
		slog.Error(err.Error())
		return nil, fmt.Errorf("user validation failed: %w", err)
	}

	userId, returnedUsername, err := s.repo.CreateUser(ctx, user.Username, user.Password, user.Email)
	if err != nil {
		slog.Error("create user failed", "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	newUser := &pkg.User{
		ID:       userId,
		Username: returnedUsername,
	}

	return newUser, nil
}

func (s *Service) GetUserByEmail(ctx context.Context, email string) (*pkg.User, error) {
	slog.Debug("looking for user", "email", email)
	user, err := s.repo.GetUser(ctx, email)
	if err != nil {
		slog.Error("user "+email+" not found", "error", err)
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
