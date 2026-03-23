package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
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
	Host     string `yaml:"host"     validate:"required"`
	Port     int    `yaml:"port"     validate:"required,min=1,max=65535"`
	Name     string `yaml:"name"     validate:"required"`
	User     string `yaml:"user"     validate:"required"`
	Password string `yaml:"password"`
}

type JWTConfig struct {
	Secret       string `yaml:"secret"        validate:"required,min=32"`
	ExpiresHours int    `yaml:"expires_hours" validate:"required,min=1,max=8760"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
	DB     DBConfig     `yaml:"db"`
	JWT    JWTConfig    `yaml:"jwt"`
}

var (
	globalConfig *Config
	once         sync.Once
	validate     = validator.New()
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
			Host: "localhost",
			Port: 5432,
			Name: "mrfood",
			User: "postgres",
		},
		JWT: JWTConfig{
			Secret:       "to-be-saved",
			ExpiresHours: 24,
		},
	}

	overrideWithEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	slog.Info("config loaded",
		slog.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		slog.String("db", fmt.Sprintf("%s:%d/%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)),
		slog.String("log_level", cfg.Log.Level),
		slog.Int("jwt_expires_hours", cfg.JWT.ExpiresHours),
	)

	return cfg, nil
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

func overrideWithEnv(cfg *Config) {
	cfg.Server.Host = getEnv("APP_SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = getEnvInt("APP_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = getEnvDuration("APP_SERVER_TIMEOUT", cfg.Server.Timeout)

	cfg.DB.Host = getEnv("DB_HOST", cfg.DB.Host)
	cfg.DB.Port = getEnvInt("DB_PORT", cfg.DB.Port)
	cfg.DB.Name = getEnv("DB_NAME", cfg.DB.Name)
	cfg.DB.User = getEnv("DB_USER", cfg.DB.User)
	cfg.DB.Password = getEnv("DB_PASS", cfg.DB.Password)

	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)

	cfg.JWT.Secret = getEnv("APP_JWT_SECRET", cfg.JWT.Secret)
	cfg.JWT.ExpiresHours = getEnvInt("APP_JWT_EXPIRES_HOURS", cfg.JWT.ExpiresHours)
}

func validateConfig(cfg *Config) error {
	if err := validate.Struct(cfg); err != nil {
		var errs validator.ValidationErrors
		if errors.As(err, &errs) {
			msgs := make([]string, 0, len(errs))
			for _, e := range errs {
				msgs = append(msgs, fmt.Sprintf("%s: failed '%s'", e.Field(), e.Tag()))
			}
			return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
		}
		return err
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
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		slog.Warn("invalid int env var, using default",
			slog.String("key", key),
			slog.String("value", value),
			slog.Int("default", defaultValue),
		)
		return defaultValue
	}
	return intVal
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		slog.Warn("invalid duration env var, using default",
			slog.String("key", key),
			slog.String("value", value),
			slog.String("default", defaultValue.String()),
		)
		return defaultValue
	}
	return d
}
