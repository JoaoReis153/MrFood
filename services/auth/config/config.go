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

// KeycloakConfig holds all settings needed to talk to a Keycloak instance.
type KeycloakConfig struct {
	// BaseURL is the root URL of Keycloak, e.g. http://keycloak:8080
	BaseURL string `yaml:"base_url" validate:"required"`
	// Realm is the Keycloak realm name, e.g. mrfood
	Realm string `yaml:"realm" validate:"required"`
	// ClientID is the confidential client used for token operations
	ClientID string `yaml:"client_id" validate:"required"`
	// ClientSecret is the client secret for the confidential client
	ClientSecret string `yaml:"client_secret" validate:"required"`
	// AdminUser / AdminPass are master-realm admin credentials for the Admin API
	AdminUser string `yaml:"admin_user" validate:"required"`
	AdminPass string `yaml:"admin_pass" validate:"required"`
}

type NotificationConfig struct {
	GRPCAddr string `yaml:"grpc_addr" validate:"required"`
}

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Log          LogConfig          `yaml:"log"`
	Keycloak     KeycloakConfig     `yaml:"keycloak"`
	Notification NotificationConfig `yaml:"notification"`
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
	}

	overrideWithEnv(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	slog.Debug("config loaded",
		slog.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		slog.String("keycloak_url", cfg.Keycloak.BaseURL),
		slog.String("keycloak_realm", cfg.Keycloak.Realm),
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
	cfg.Server.Port = getEnvInt("AUTH_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = getEnvDuration("APP_SERVER_TIMEOUT", cfg.Server.Timeout)

	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)

	cfg.Keycloak.BaseURL = getEnv("KEYCLOAK_BASE_URL", cfg.Keycloak.BaseURL)
	cfg.Keycloak.Realm = getEnv("KEYCLOAK_REALM", cfg.Keycloak.Realm)
	cfg.Keycloak.ClientID = getEnv("KEYCLOAK_CLIENT_ID", cfg.Keycloak.ClientID)
	cfg.Keycloak.ClientSecret = getEnv("KEYCLOAK_CLIENT_SECRET", cfg.Keycloak.ClientSecret)
	cfg.Keycloak.AdminUser = getEnv("KEYCLOAK_ADMIN_USER", cfg.Keycloak.AdminUser)
	cfg.Keycloak.AdminPass = getEnv("KEYCLOAK_ADMIN_PASS", cfg.Keycloak.AdminPass)

	cfg.Notification.GRPCAddr = getEnv("AUTH_TO_NOTIFICATION_GRPC_ADDR", cfg.Notification.GRPCAddr)
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
