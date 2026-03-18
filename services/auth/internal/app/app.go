package app

import (
	"MrFood/services/auth/internal/repository"
	"MrFood/services/auth/internal/service"
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
