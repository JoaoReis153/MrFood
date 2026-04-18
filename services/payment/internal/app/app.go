package app

import (
	"MrFood/services/payment/config"
	"MrFood/services/payment/internal/repository"
	"MrFood/services/payment/internal/service"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type App struct {
	Service          *service.Service
	Repo             *repository.Repository
	DB               *pgxpool.Pool
	NotificationConn *grpc.ClientConn
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

	client, notificationConn, err := NewClient(cfg.Notification.GRPCAddr)
	svc := service.New(repo, client)

	return &App{
		Repo:             repo,
		Service:          svc,
		DB:               conns.DB,
		NotificationConn: notificationConn,
	}, nil
}

func (a *App) Close(ctx context.Context) error {
	if a.Repo == nil {
		return nil
	}
	return a.Repo.Close(ctx)
}
