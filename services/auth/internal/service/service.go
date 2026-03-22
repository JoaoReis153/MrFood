package service

import (
	"MrFood/services/auth/internal/repository"
	models "MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) StoreUser(ctx context.Context, user *models.User) (*models.User, error) {
	if !validEmail(user.Email) {
		slog.Error("invalid email format")
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("hash password failed", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userId, returnedUsername, err := s.repo.CreateUser(ctx, user.Username, string(hashedPassword), user.Email)
	if err != nil {
		slog.Error("create user failed", "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	newUser := &models.User{
		ID:       userId,
		Username: returnedUsername,
	}

	return newUser, nil
}

func validEmail(email string) bool {
	if email == "" || len(email) > 254 || !emailRegex.MatchString(email) {
		return false
	}
	return true
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func (s *Service) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	email = strings.TrimSpace(email)
	if !validEmail(email) {
		slog.Error("invalid email format")
		return nil, fmt.Errorf("invalid email format")
	}

	user, err := s.repo.GetUser(ctx, email)
	if err != nil {
		slog.Error("user not found", "error", err)
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
