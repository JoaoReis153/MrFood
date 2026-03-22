package main

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/app"
	"log"
)

func main() {
	config.Get()

	app := app.New()

	err := app.ConnectDb()
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer app.DB.Close()

	app.InitDependencies()

	app.RunServer()
}
