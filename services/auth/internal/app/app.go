package app

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/repository"
	"MrFood/services/auth/internal/service"
	"context"
	"fmt"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	db, err := repository.NewDBPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	redis, err := repository.NewRedisClient(ctx, cfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}

	repo := repository.New(db, redis)
	svc := service.New(repo)

	return &App{
		Repo:    repo,
		Service: svc,
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	var errs []error
	if a.Repo != nil {
		if err := a.Repo.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
