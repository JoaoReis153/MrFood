package app

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/keycloak"
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	KC               *keycloak.Client
	notificationConn *grpc.ClientConn
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	kc := keycloak.New(
		cfg.Keycloak.BaseURL,
		cfg.Keycloak.Realm,
		cfg.Keycloak.ClientID,
		cfg.Keycloak.ClientSecret,
		cfg.Keycloak.AdminUser,
		cfg.Keycloak.AdminPass,
	)

	conn, err := grpc.NewClient(cfg.Notification.GRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("notification grpc: %w", err)
	}

	return &App{
		KC:               kc,
		notificationConn: conn,
	}, nil
}

func (a *App) Close(_ context.Context) error {
	if a.notificationConn != nil {
		return a.notificationConn.Close()
	}
	return nil
}
