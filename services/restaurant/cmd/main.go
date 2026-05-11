package main

import (
	"MrFood/services/restaurant/config"
	"MrFood/services/restaurant/internal/app"
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
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

	shutdown, err := initTracer(ctx, "mrfood-restaurant")
	if err != nil {
		slog.Error("tracer init failed", "error", err)
	}
	defer shutdown()

	app := app.New()
	defer app.Close()

	if err := app.ConnectDb(); err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer app.DB.Close()

	app.InitDependencies()
	app.RunServer()
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

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			slog.Error("tracer shutdown failed", "error", err)
		}
	}, nil
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
