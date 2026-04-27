package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server struct {
		Host    string        `yaml:"host"`
		Port    int           `yaml:"port"`
		Timeout time.Duration `yaml:"timeout"`
	} `yaml:"server"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	DB struct {
		Host              string        `yaml:"host"`
		Port              int           `yaml:"port"`
		Name              string        `yaml:"name"`
		User              string        `yaml:"user"`
		Password          string        `yaml:"password"`
		MinConns          int32         `yaml:"min_conns"`
		MaxConns          int32         `yaml:"max_conns"`
		MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime"`
		HealthCheckPeriod time.Duration `yaml:"health_check_period"`
	} `yaml:"db"`
	Review struct {
		GRPCAddr string `yaml:"grpc_addr"`
	} `yaml:"review"`
}

var globalConfig *Config

func Load(ctx context.Context) *Config {
	cfg := &Config{
		Server: struct {
			Host    string        `yaml:"host"`
			Port    int           `yaml:"port"`
			Timeout time.Duration `yaml:"timeout"`
		}{
			Host:    "0.0.0.0",
			Port:    8080,
			Timeout: 30 * time.Second,
		},
		Log: struct {
			Level string `yaml:"level"`
		}{
			Level: "info",
		},
		DB: struct {
			Host              string        `yaml:"host"`
			Port              int           `yaml:"port"`
			Name              string        `yaml:"name"`
			User              string        `yaml:"user"`
			Password          string        `yaml:"password"`
			MinConns          int32         `yaml:"min_conns"`
			MaxConns          int32         `yaml:"max_conns"`
			MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime"`
			HealthCheckPeriod time.Duration `yaml:"health_check_period"`
		}{
			Host:              "localhost",
			Port:              5432,
			Name:              "mrfood",
			User:              "postgres",
			Password:          "",
			MinConns:          4,
			MaxConns:          20,
			MaxConnLifetime:   15 * time.Minute,
			HealthCheckPeriod: 1 * time.Minute,
		},
		Review: struct {
			GRPCAddr string `yaml:"grpc_addr"`
		}{
			GRPCAddr: "localhost:50055",
		},
	}

	overrideWithEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid config", "error", err)
		panic(err)
	}

	slog.Info("config loaded",
		slog.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		slog.String("db", fmt.Sprintf("%s:%d/%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)),
		slog.String("log_level", cfg.Log.Level),
	)

	return cfg
}

func overrideWithEnv(cfg *Config) {
	cfg.Server.Host = getEnv("APP_SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = getEnvInt("RESTAURANT_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = parseDuration(getEnv("APP_SERVER_TIMEOUT", "30s"))

	cfg.DB.Host = getEnv("POSTGRES_HOST", cfg.DB.Host)
	cfg.DB.Port = getEnvInt("POSTGRES_PORT", cfg.DB.Port)
	cfg.DB.Name = getEnv("RESTAURANT_POSTGRES_DB", cfg.DB.Name)
	cfg.DB.User = getEnv("RESTAURANT_POSTGRES_USER", cfg.DB.User)
	cfg.DB.Password = getEnv("RESTAURANT_POSTGRES_PASSWORD", cfg.DB.Password)
	cfg.DB.MinConns = int32(getEnvInt("POSTGRES_MIN_CONNS", int(cfg.DB.MinConns)))
	cfg.DB.MaxConns = int32(getEnvInt("POSTGRES_MAX_CONNS", int(cfg.DB.MaxConns)))
	cfg.DB.MaxConnLifetime = parseDuration(getEnv("POSTGRES_MAX_CONN_LIFETIME", "15m"))
	cfg.DB.HealthCheckPeriod = parseDuration(getEnv("POSTGRES_HEALTH_CHECK_PERIOD", "1m"))
	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)
	cfg.Review.GRPCAddr = getEnv("REVIEW_GRPC_ADDR", cfg.Review.GRPCAddr)

}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port invalid: %d", cfg.Server.Port)
	}
	if cfg.Server.Timeout == 0 {
		return fmt.Errorf("server timeout required")
	}
	if cfg.DB.Host == "" || cfg.DB.Name == "" {
		return fmt.Errorf("DB host/name required")
	}
	if strings.TrimSpace(cfg.Review.GRPCAddr) == "" {
		return fmt.Errorf("review grpc addr required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	// Support "30s", "30", "30s" etc.
	s = strings.TrimSuffix(s, "s")
	if d, err := time.ParseDuration(s + "s"); err == nil {
		return d
	}
	return 30 * time.Second
}

func Get(ctx context.Context) *Config {
	if globalConfig == nil {
		globalConfig = Load(ctx)
	}
	return globalConfig
}
