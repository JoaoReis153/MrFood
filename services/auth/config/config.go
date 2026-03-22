package config

import (
	"log"
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
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"db"`
}

var globalConfig *Config

// Load config from ENV vars only (with sane defaults)
func Load() *Config {
	cfg := &Config{
		// Hardcoded defaults (non-zero!)
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
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Name     string `yaml:"name"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
		}{
			Host: "localhost",
			Port: 5432,
		},
	}

	overrideWithEnv(cfg)

	log.Printf("Config loaded: Server=%s:%d, DB=%s:%d/%s, Log=%s",
		cfg.Server.Host, cfg.Server.Port,
		cfg.DB.Host, cfg.DB.Port, cfg.DB.Name,
		cfg.Log.Level,
	)

	return cfg
}

func overrideWithEnv(cfg *Config) {
	// Server config
	cfg.Server.Host = getEnv("APP_SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = getEnvInt("APP_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Timeout = parseDuration(getEnv("APP_SERVER_TIMEOUT", "30s"))
	cfg.DB.Host = getEnv("DB_HOST", cfg.DB.Host)
	cfg.DB.Port = getEnvInt("DB_PORT", cfg.DB.Port)
	cfg.DB.Name = getEnv("DB_NAME", cfg.DB.Name)
	cfg.DB.User = getEnv("DB_USER", cfg.DB.User)
	cfg.DB.Password = getEnv("DB_PASS", cfg.DB.Password)
	cfg.Log.Level = getEnv("APP_LOG_LEVEL", cfg.Log.Level)
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

func Get() *Config {
	if globalConfig == nil {
		globalConfig = Load()
	}
	return globalConfig
}
