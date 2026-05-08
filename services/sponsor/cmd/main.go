package main

import (
	"MrFood/services/sponsor/config"
	"MrFood/services/sponsor/internal/app"
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx := context.Background()
	setupLogger(config.Get(ctx).Log.Level)

	shutdown, err := initTracer(ctx, "mrfood-sponsor")
	if err != nil {
		slog.Error("tracer init failed", "error", err)
	}
	defer shutdown()

	shutdownMeter, err := initMeter(ctx, "mrfood-sponsor")
	if err != nil {
		slog.Error("meter init failed", "error", err)
	}
	defer shutdownMeter()

	application := app.New()
	defer application.Close()

	if err := application.ConnectDb(); err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer application.DB.Close()

	application.InitDependencies()
	application.RunServer()
}

func initMeter(ctx context.Context, serviceName string) (func(), error) {
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint("otel-collector:4317"),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetMeterProvider(mp)

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mp.Shutdown(shutdownCtx); err != nil {
			slog.Error("meter shutdown failed", "error", err)
		}
	}, nil
}

func initTracer(ctx context.Context, serviceName string) (func(), error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("otel-collector:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
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
