package app

import (
	"MrFood/services/booking/internal/repository"
	"MrFood/services/booking/internal/service"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
	DB      *pgxpool.Pool
}

func New() *App {
	return &App{}
}

func (app *App) InitDependencies() {
	if app.DB == nil {
		panic("DB not initialized")
	}

	client, _, err := NewClient("localhost:50053")
	if err != nil {
		panic(fmt.Errorf("client init failed: %w", err))
	}

	app.Repo = repository.New(app.DB)
	app.Service = service.New(app.Repo, client)
}
