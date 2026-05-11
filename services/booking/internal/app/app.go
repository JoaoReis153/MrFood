package app

import (
	"MrFood/services/booking/config"
	"MrFood/services/booking/internal/repository"
	"MrFood/services/booking/internal/service"
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
	PaymentConn    *grpc.ClientConn
}

func New() *App {
	return &App{}
}

func (app *App) InitDependencies() {
	if app.DB == nil {
		panic("DB not initialized")
	}

	cfg := config.Get(context.Background())

	restaurantClient, restaurant_conn, err := NewRestaurantClient(cfg.Restaurant.GRPCAddr)
	if err != nil {
		panic(fmt.Errorf("client init failed: %w", err))
	}

	paymentClient, paymentConn, err := NewPaymentClient(cfg.Payment.GRPCAddr)
	if err != nil {
		panic(fmt.Errorf("client init failed: %w", err))
	}

	app.RestaurantConn = restaurant_conn
	app.PaymentConn = paymentConn
	app.Repo = repository.New(app.DB)
	app.Service = service.New(app.Repo, restaurantClient, paymentClient)
}

func (app *App) Close() {
	if app.RestaurantConn != nil {
		_ = app.RestaurantConn.Close()
	}
}
