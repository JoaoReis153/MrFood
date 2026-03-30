package app

import (
	"MrFood/services/sponsor/config"
	"MrFood/services/sponsor/internal/repository"
	"MrFood/services/sponsor/internal/service"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type App struct {
	Service        *service.Service
	Repo           *repository.Repository
	DB             *pgxpool.Pool
	RestaurantConn *grpc.ClientConn
}

func New() *App {
	return &App{}
}

func (app *App) InitDependencies() {
	if app.DB == nil {
		panic("DB not initialized")
	}
	cfg := config.Get(context.Background())

	client, restaurant_conn, err := NewClient(cfg.Restaurant.GRPCAddr)
	if err != nil {
		panic(fmt.Errorf("client init failed: %w", err))
	}

	app.RestaurantConn = restaurant_conn
	app.Repo = repository.New(app.DB)
	app.Service = service.New(app.Repo, client)
}

func (app *App) Close() {
	if app.RestaurantConn != nil {
		_ = app.RestaurantConn.Close()
	}
}
