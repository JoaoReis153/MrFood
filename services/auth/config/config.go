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

type RedisConfig struct {
	Host     string `yaml:"host"     validate:"required"`
	Port     int    `yaml:"port"     validate:"required,min=1,max=65535"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"       validate:"min=0"`
}

type JWTConfig struct {
	AccessTokenSecret  string        `yaml:"secret"        validate:"required,min=32"`
	RefreshTokenSecret string        `yaml:"refresh_secret" validate:"required,min=32"`
	AccessTokenTTL     time.Duration `yaml:"access_token_ttl" validate:"required,min=5m,max=2h"`
	RefreshTokenTTL    time.Duration `yaml:"refresh_token_ttl" validate:"required,min=1h,max=720h"`
}

type Config struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
	DB     DBConfig     `yaml:"db"`
	Redis  RedisConfig  `yaml:"redis"`
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
			Port:    50051,
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
			MinConns:          4,
			MaxConns:          20,
			MaxConnLifetime:   15 * time.Minute,
			HealthCheckPeriod: 1 * time.Minute,
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		JWT: JWTConfig{
			AccessTokenSecret:  "to-be-saved",
			RefreshTokenSecret: "to-be-saved",
			AccessTokenTTL:     15 * time.Minute,
			RefreshTokenTTL:    7 * 24 * time.Hour,
		},
	}

	overrideWithEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	slog.Info("config loaded",
		slog.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		slog.String("db", fmt.Sprintf("%s:%d/%s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)),
		slog.String("redis", fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)),
		slog.String("log_level", cfg.Log.Level),
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
	cfg.Server.Timeout = getEnvDuration("APP_SERVER_TIMEOUT", cfg.Server.Timeout)

	cfg.DB.Host = getEnvAny(cfg.DB.Host, "DB_HOST")
	cfg.DB.Name = getEnvAny(cfg.DB.Name, "POSTGRES_DB")
	cfg.DB.User = getEnvAny(cfg.DB.User, "POSTGRES_USER")
	cfg.DB.Password = getEnvAny(cfg.DB.Password, "POSTGRES_PASSWORD")
	cfg.DB.MinConns = getEnvInt32("DB_MIN_CONNS", cfg.DB.MinConns)
	cfg.DB.MaxConns = getEnvInt32("DB_MAX_CONNS", cfg.DB.MaxConns)
	cfg.DB.MaxConnLifetime = getEnvDuration("DB_MAX_CONN_LIFETIME", cfg.DB.MaxConnLifetime)
	cfg.DB.HealthCheckPeriod = getEnvDuration("DB_HEALTH_CHECK_PERIOD", cfg.DB.HealthCheckPeriod)

	cfg.Redis.Host = getEnvAny(cfg.Redis.Host, "REDIS_HOST", "AUTH_REDIS_HOST")
	cfg.Redis.Password = getEnvAny(cfg.Redis.Password, "REDIS_PASS", "AUTH_REDIS_PASS")
	cfg.Redis.DB = getEnvInt("REDIS_DB", cfg.Redis.DB)

	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)

	cfg.JWT.AccessTokenSecret = getEnvAny(cfg.JWT.AccessTokenSecret, "APP_JWT_ACCESS_TOKEN_SECRET", "AUTH_JWT_ACCESS_TOKEN_SECRET")
	cfg.JWT.RefreshTokenSecret = getEnvAny(cfg.JWT.RefreshTokenSecret, "APP_JWT_REFRESH_TOKEN_SECRET", "AUTH_JWT_REFRESH_TOKEN_SECRET")
}

func validateConfig(cfg *Config) error {
	if err := validate.Struct(cfg); err != nil {
		var errs validator.ValidationErrors
		if errors.As(err, &errs) {
			msgs := make([]string, 0, len(errs))
			for _, e := range errs {
				msgs = append(msgs, fmt.Sprintf("%s: failed '%s'", e.Namespace(), e.Tag()))
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

func getEnvAny(defaultValue string, keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
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

func getEnvIntAny(defaultValue int, keys ...string) int {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
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
	}
	return defaultValue
}

func getEnvInt32(key string, defaultValue int32) int32 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		slog.Warn("invalid int32 env var, using default",
			slog.String("key", key),
			slog.String("value", value),
			slog.Int("default", int(defaultValue)),
		)
		return defaultValue
	}
	return int32(intVal)
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
