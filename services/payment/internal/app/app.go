package app

import (
	"MrFood/services/payment/config"
	"MrFood/services/payment/internal/repository"
	"MrFood/services/payment/internal/service"
	"context"
	"fmt"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	repo, err := repository.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("repository: %w", err)
	}

	svc := service.New(repo)

	return &App{
		Repo:    repo,
		Service: svc,
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	if a.Repo == nil {
		return nil
	}
	return a.Repo.Close(ctx)
}
