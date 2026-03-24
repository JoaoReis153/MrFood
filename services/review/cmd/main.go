package main

import (
	"MrFood/services/review/config"
	"MrFood/services/review/internal/app"
	"MrFood/services/review/internal/db"
	"MrFood/services/review/internal/repository"
	"MrFood/services/review/internal/service"
	"log"
)

func main() {
	cfg := config.Load()

	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	repo := repository.New(database)
	svc := service.New(repo, repo)

	app.RunServer(svc)
}
