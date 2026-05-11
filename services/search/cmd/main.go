package main

import (
	"MrFood/services/search/config"
	"MrFood/services/search/internal/app"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	cfg := config.Get(ctx)
	setupLogger(cfg.Log.Level)

	app, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("app init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := app.Close(ctx); err != nil {
			slog.Error("app close failed", "error", err)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting server")
	if err := app.RunServer(shutdownCtx, cfg); err != nil {
		slog.Error("server failed", "error", err)
	}
}

func setupLogger(logLevel string) {
	level := slog.LevelInfo

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
