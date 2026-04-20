package app

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/repository"
	"MrFood/services/auth/internal/service"
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	Service          *service.Service
	Repo             *repository.Repository
	notificationConn *grpc.ClientConn
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

	conn, err := grpc.NewClient(cfg.Notification.GRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("notification grpc: %w", err)
	}

	repo := repository.New(db, redis)
	svc := service.New(repo)

	return &App{
		Repo:             repo,
		Service:          svc,
		notificationConn: conn,
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	var errs []error
	if a.Repo != nil {
		if err := a.Repo.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if a.notificationConn != nil {
		if err := a.notificationConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
