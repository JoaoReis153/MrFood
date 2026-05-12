package main

import (
	"MrFood/services/search/config"
	"MrFood/services/search/internal/app"
	"MrFood/services/search/internal/telemetry"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	cfg := config.Get(ctx)

	shutdownTelemetry, err := telemetry.Setup(ctx, "mrfood-search", telemetry.ParseLevel(cfg.Log.Level))
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry setup failed: %v\n", err)
	}
	defer shutdownTelemetry()

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
