package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ServerConfig struct {
	Host    string        `yaml:"host"    validate:"required"`
	Port    int           `yaml:"port"    validate:"required,min=1,max=65535"`
	Timeout time.Duration `yaml:"timeout" validate:"required"`
}

type LogConfig struct {
	Level string `yaml:"level" validate:"required,oneof=debug info warn error"`
}

type DBConfig struct {
	Host              string `yaml:"host"     validate:"required"`
	Port              int    `yaml:"port"     validate:"required,min=1,max=65535"`
	Name              string `yaml:"name"     validate:"required"`
	User              string `yaml:"user"     validate:"required"`
	Password          string `yaml:"password"`
	MinConns          int32
	MaxConns          int32         `yaml:"max_conns"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
	DB     DBConfig     `yaml:"db"`
}

var (
	globalConfig *Config
	once         sync.Once
)

func Load(_ context.Context) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:    "0.0.0.0",
			Port:    8080,
			Timeout: 30 * time.Second,
		},
		Log: LogConfig{
			Level: "info",
		},
		DB: DBConfig{
			Host:              "localhost",
			Port:              5432,
			Name:              "mrfood",
			User:              "postgres",
			Password:          "",
			MinConns:          2,
			MaxConns:          20,
			MaxConnLifetime:   15 * time.Minute,
			HealthCheckPeriod: 1 * time.Minute,
		},
	}

	// Override with ENV vars
	overrideWithEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid config", "error", err)
		return nil, fmt.Errorf("config: %w", err)
	}

	slog.Info("config loaded",
		slog.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		slog.String("db", fmt.Sprintf("%s:%d/%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)),
		slog.String("log_level", cfg.Log.Level),
	)

	return cfg, nil
}

func overrideWithEnv(cfg *Config) {
	// Server config
	cfg.Server.Host = getEnv("APP_SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = getEnvInt("APP_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = parseDuration(getEnv("APP_SERVER_TIMEOUT", "30s"))

	// Log config
	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)

	// Database config
	cfg.DB.Host = getEnv("DB_HOST", cfg.DB.Host)
	cfg.DB.Port = getEnvInt("DB_PORT", cfg.DB.Port)
	cfg.DB.Name = getEnv("DB_NAME", cfg.DB.Name)
	cfg.DB.User = getEnv("DB_USER", cfg.DB.User)
	cfg.DB.Password = getEnv("DB_PASSWORD", cfg.DB.Password)
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
	if seconds, err := strconv.Atoi(s); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return 30 * time.Second
}

func Get(ctx context.Context) *Config {
	once.Do(func() {
		cfg, err := Load(ctx)
		if err != nil {
			slog.Error("invalid config", "error", err)
			panic(err)
		}
		globalConfig = cfg
	})
	return globalConfig
}
