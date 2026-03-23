package main

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/app"
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
)

func main() {
	setupLogger(config.Get(context.Background()).Log.Level)

	config.Get(context.Background())

	app := app.New()

	err := app.ConnectDb()
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer app.DB.Close()

	app.InitDependencies()

	app.RunServer()
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
		AddSource: true,
	})

	slog.SetDefault(slog.New(handler))
	slog.Info("logger initialized", "level", logLevel)
}
