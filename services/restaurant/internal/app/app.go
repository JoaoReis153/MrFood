package app

import (
	"MrFood/services/restaurant/internal/repository"
	"MrFood/services/restaurant/internal/service"
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
