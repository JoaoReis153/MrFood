package app

import (
	"MrFood/services/gateway/internal/repository"
	"MrFood/services/gateway/internal/service"
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
