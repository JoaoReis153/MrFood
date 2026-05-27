package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const collectorEndpoint = "otel-collector:4317"

// Setup initialises traces, metrics and logs, wiring all three to the OTel
// Collector. Returns a single shutdown function that must be deferred by the
// caller.
func Setup(ctx context.Context, serviceName string, level slog.Level) (func(), error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return func() {}, fmt.Errorf("build otel resource: %w", err)
	}

	shutdownTracer, err := setupTracer(ctx, res)
	if err != nil {
		return func() {}, fmt.Errorf("setup tracer: %w", err)
	}

	shutdownMeter, err := setupMeter(ctx, res)
	if err != nil {
		shutdownTracer()
		return func() {}, fmt.Errorf("setup meter: %w", err)
	}

	shutdownLogger, err := setupLogger(ctx, res, serviceName, level)
	if err != nil {
		shutdownMeter()
		shutdownTracer()
		return func() {}, fmt.Errorf("setup logger: %w", err)
	}

	return func() {
		shutdownLogger()
		shutdownMeter()
		shutdownTracer()
	}, nil
}

// ParseLevel converts a log level string to slog.Level, defaulting to Info.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupTracer(ctx context.Context, res *resource.Resource) (func(), error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collectorEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "tracer shutdown failed: %v\n", err)
		}
	}, nil
}

func setupMeter(ctx context.Context, res *resource.Resource) (func(), error) {
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(collectorEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mp.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "meter shutdown failed: %v\n", err)
		}
	}, nil
}

func setupLogger(ctx context.Context, res *resource.Resource, serviceName string, level slog.Level) (func(), error) {
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(collectorEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return func() {}, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	stderrHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	slog.SetDefault(slog.New(&multiHandler{
		handlers: []slog.Handler{
			stderrHandler,
			otelslog.NewHandler(serviceName),
		},
	}))

	slog.Info("telemetry initialised", "service", serviceName, "log_level", level.String())

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := lp.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "logger shutdown failed: %v\n", err)
		}
	}, nil
}

// multiHandler fans out a slog.Record to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
