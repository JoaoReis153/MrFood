package app

import (
	"MrFood/services/restaurant/internal/repository"
	"MrFood/services/restaurant/internal/service"

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

	app.Repo = repository.New(app.DB)
	app.Service = service.New(app.Repo)
}
