package app

import (
	"MrFood/services/review/internal/repository"
	"MrFood/services/review/internal/service"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
}

func New() *App {
	repo := repository.New()
	svc := service.New(*repo)

	return &App{
		Service: svc,
	}
}
