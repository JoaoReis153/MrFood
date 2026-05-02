package main

import (
	"MrFood/services/auth/config"
	"MrFood/services/auth/internal/app"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx := context.Background()
	cfg := config.Get(ctx)
	setupLogger(cfg.Log.Level)

	shutdown, err := initTracer(ctx, "mrfood-auth")
	if err != nil {
		slog.Error("tracer init failed", "error", err)
	}
	defer shutdown()

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

func initTracer(ctx context.Context, serviceName string) (func(), error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("otel-collector:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() { tp.Shutdown(ctx) }, nil
}
