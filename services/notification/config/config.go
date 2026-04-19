package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"
)

type ServerConfig struct {
	Port    int           `yaml:"port"    validate:"required,min=1,max=65535"`
	Timeout time.Duration `yaml:"timeout" validate:"required"`
}

type LogConfig struct {
	Level string `yaml:"level" validate:"required,oneof=debug info warn error"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

type RateLimitConfig struct {
	EmailRateLimit int           `yaml:"email_rate_limit"`
	TTL            time.Duration `yaml:"rate_limit_ttl"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Log       LogConfig       `yaml:"log"`
	Redis     RedisConfig     `yaml:"redis"`
	SMTP      SMTPConfig      `yaml:"smtp"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

var (
	globalConfig *Config
	once         sync.Once
)

func Load(_ context.Context) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:    0,
			Timeout: 30 * time.Second,
		},
		Log: LogConfig{
			Level: "info",
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
		},
		SMTP: SMTPConfig{
			Host: "localhost",
			Port: 587,
		},
		RateLimit: RateLimitConfig{
			EmailRateLimit: 5,
			TTL:            24 * time.Hour,
		},
	}

	overrideWithEnv(cfg)

	if cfg.Server.Port == 0 {
		return nil, fmt.Errorf("config: APP_SERVER_PORT is required")
	}
	if cfg.SMTP.User == "" {
		return nil, fmt.Errorf("config: SMTP_USER is required")
	}
	if cfg.SMTP.Password == "" {
		return nil, fmt.Errorf("config: SMTP_PASSWORD is required")
	}
	if cfg.SMTP.From == "" {
		return nil, fmt.Errorf("config: SMTP_FROM is required")
	}

	slog.Info("config loaded",
		slog.String("port", fmt.Sprintf("%d", cfg.Server.Port)),
		slog.String("log_level", cfg.Log.Level),
		slog.String("redis", fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)),
	)

	return cfg, nil
}

func overrideWithEnv(cfg *Config) {
	cfg.Server.Port = getEnvInt("APP_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = getEnvDuration("APP_SERVER_TIMEOUT", cfg.Server.Timeout)
	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)
	cfg.Redis.Host = getEnv("REDIS_HOST", cfg.Redis.Host)
	cfg.Redis.Port = getEnvInt("REDIS_PORT", cfg.Redis.Port)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", cfg.Redis.Password)
	cfg.SMTP.Host = getEnv("SMTP_HOST", cfg.SMTP.Host)
	cfg.SMTP.Port = getEnvInt("SMTP_PORT", cfg.SMTP.Port)
	cfg.SMTP.User = getEnv("SMTP_USER", cfg.SMTP.User)
	cfg.SMTP.Password = getEnv("SMTP_PASSWORD", cfg.SMTP.Password)
	cfg.SMTP.From = getEnv("SMTP_FROM", cfg.SMTP.From)
	cfg.RateLimit.EmailRateLimit = getEnvInt("RATE_LIMIT_EMAIL", cfg.RateLimit.EmailRateLimit)
	cfg.RateLimit.TTL = getEnvDuration("RATE_LIMIT_TTL", cfg.RateLimit.TTL)
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
		slog.Warn("invalid int env var, using default", slog.String("key", key))
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
		slog.Warn("invalid duration env var, using default", slog.String("key", key))
		return defaultValue
	}
	return d
}
