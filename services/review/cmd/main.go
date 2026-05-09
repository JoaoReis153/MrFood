package main

import (
	"MrFood/services/review/config"
	"MrFood/services/review/internal/app"
	"MrFood/services/review/internal/telemetry"
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	cfg := config.Get(ctx)

	shutdownTelemetry, err := telemetry.Setup(ctx, "mrfood-review", telemetry.ParseLevel(cfg.Log.Level))
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry setup failed: %v\n", err)
	}
	defer shutdownTelemetry()

	application := app.New()
	defer application.Close()

	if err := application.ConnectDb(); err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer application.DB.Close()

	application.InitDependencies()
	application.RunServer()
}
