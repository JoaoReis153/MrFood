package app

import (
	"MrFood/services/payment/config"
	"MrFood/services/payment/internal/repository"
	"MrFood/services/payment/internal/service"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
	DB      *pgxpool.Pool
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	conns, err := repository.NewConnections(ctx, cfg)

	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	repo, err := repository.New(ctx, cfg, conns.DB)
	if err != nil {
		return nil, fmt.Errorf("repository: %w", err)
	}

	svc := service.New(repo)

	return &App{
		Repo:    repo,
		Service: svc,
		DB:      conns.DB,
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	if a.Repo == nil {
		return nil
	}
	return a.Repo.Close(ctx)
}
