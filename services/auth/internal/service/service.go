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

func (s *Service) StoreUser(ctx context.Context, user *models.User) (*models.User, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if !validEmail(user.Email) {
		slog.Error("invalid email format")
		return nil, status.Error(codes.InvalidArgument, "invalid email")
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
	user, err := s.repo.GetUser(ctx, email)
	if err != nil {
		slog.Error("user not found", "error", err)
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
