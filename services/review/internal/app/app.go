package app

import (
	"MrFood/services/review/internal/repository"
	"MrFood/services/review/internal/service"
	"database/sql"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
}

func New(db *sql.DB) *App {
	repo := repository.New(db)
	svc := service.New(*repo)
	return &App{
		Service: svc,
		Repo:    repo,
	}
}
