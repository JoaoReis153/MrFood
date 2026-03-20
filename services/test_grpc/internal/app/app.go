package app

import (
	"MrFood/services/test_grpc/internal/repository"
	"MrFood/services/test_grpc/internal/service"
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
