package service

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/auth"
	"MrFood/services/auth/internal/repository"
	"MrFood/services/auth/pkg"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicateUser = errors.New("user already exists")

type Service struct {
	repo       userStore
	jwtService *auth.JWTService
}

type userStore interface {
	CreateUser(ctx context.Context, username, password, email string) (int64, string, error)
	GetUser(ctx context.Context, email string) (*pkg.User, error)
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo, jwtService: auth.NewJWTService(&config.Get(context.Background()).JWT, repo)}
}

func (s *Service) StoreUser(ctx context.Context, user *pkg.User) (*pkg.User, error) {
	if err := pkg.ValidateUser(*user); err != nil {
		slog.Error(err.Error())
		return nil, fmt.Errorf("user validation failed: %w", err)
	}

	userId, returnedUsername, err := s.repo.CreateUser(ctx, user.Username, user.Password, user.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("user already exists")
		}
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
		slog.Error("error fetching user", "error", err)
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if user == nil {
		slog.Error("user not found", "email", email)
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}
