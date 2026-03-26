package app

import (
	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/repository"
	"MrFood/services/booking/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Service *service.Service
	Repo    *repository.Repository
	DB      *pgxpool.Pool
	Client  pb.RestaurantServiceClient
}

func New() *App {
	return &App{}
}

func (app *App) InitDependencies() {
	if app.DB == nil {
		panic("DB not initialized")
	}

	if app.Client == nil {
		panic("Client not initialized")
	}

	app.Repo = repository.New(app.DB)
	app.Service = service.New(app.Repo, app.Client)
}
