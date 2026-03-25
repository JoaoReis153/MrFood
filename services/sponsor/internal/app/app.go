package app

import (
	"MrFood/services/sponsor/internal/repository"
	"MrFood/services/sponsor/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
	DB      *pgxpool.Pool
}

func New() *App {
	repo := repository.New()
	svc := service.New(*repo)

	return &App{
		Service: svc,
	}
}

// func (app *App) InitDependencies() {
// 	if app.DB == nil {
// 		panic("DB not initialized")
// 	}

// 	app.Repo = repository.New(app.DB)
// 	app.Service = service.New(app.Repo)
// }
