package main

import (
	"MrFood/services/booking/config"
	"MrFood/services/booking/internal/api/grpc"
	"MrFood/services/booking/internal/app"
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
)

func main() {
	setupLogger(config.Get(context.Background()).Log.Level)

	config.Get(context.Background())

	application := app.New()

	err := application.ConnectDb()
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer application.DB.Close()

	client, _, err := grpc.NewClient()

	application.Client = client

	slog.Info("app client", "client", application.Client)

	application.InitDependencies()

	grpc.RunServer(application.Service)
}

func setupLogger(logLevel string) {
	level := slog.LevelInfo // Default

	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info":
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: true, // file:line numbers
	})

	slog.SetDefault(slog.New(handler))
	slog.Info("logger initialized", "level", logLevel)
}
