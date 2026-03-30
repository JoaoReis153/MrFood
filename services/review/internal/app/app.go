package app

import (
	"MrFood/services/review/config"
	"MrFood/services/review/internal/repository"
	"MrFood/services/review/internal/service"
	"context"

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
	app.Repo = repository.New(app.DB)

	restaurantClient, restaurantConn, err := NewRestaurantClient(cfg.Restaurant.GRPCAddr)
	if err != nil {
		panic("Failed to create restaurant details client")
	}

	app.RestaurantConn = restaurantConn
	app.Service = service.New(app.Repo, restaurantClient)
}

func (app *App) Close() {
	if app.RestaurantConn != nil {
		_ = app.RestaurantConn.Close()
	}
}
