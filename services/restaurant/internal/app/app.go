package app

import (
	"MrFood/services/restaurant/config"
	"MrFood/services/restaurant/internal/repository"
	"MrFood/services/restaurant/internal/service"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type App struct {
	Service    *service.Service
	Repo       *repository.Repository
	DB         *pgxpool.Pool
	ReviewConn *grpc.ClientConn
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

	reviewClient, reviewConn, err := newReviewStatsClient(cfg.Review.GRPCAddr)
	if err != nil {
		panic(fmt.Errorf("review client init failed: %w", err))
	}

	app.ReviewConn = reviewConn
	app.Service = service.New(app.Repo, reviewClient)
}

func (app *App) Close() {
	if app.ReviewConn != nil {
		_ = app.ReviewConn.Close()
	}
}
