package repository

import (
	"MrFood/services/auth/config"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func NewDBPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DB.User,
		url.QueryEscape(cfg.DB.Password),
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	slog.Debug("db config loaded",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"user", cfg.DB.User,
		"db", cfg.DB.Name,
		"pass_len", len(cfg.DB.Password),
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		slog.Error("parse config failed", "error", err)
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = cfg.DB.MaxConns
	config.MinConns = cfg.DB.MinConns
	config.MaxConnLifetime = cfg.DB.MaxConnLifetime
	config.HealthCheckPeriod = cfg.DB.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("db pool failed", "error", err)
		return nil, fmt.Errorf("db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping failed", "error", err)
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return pool, err
}

func NewRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {

	address := cfg.Redis.Host + ":" + strconv.Itoa(cfg.Redis.Port)

	slog.Debug("redis config loaded",
		"host", cfg.Redis.Host,
		"port", cfg.Redis.Port,
		"password", cfg.Redis.Password,
		"db", cfg.Redis.DB,
	)

	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("redis ping failed", "error", err)
		if err := rdb.Close(); err != nil {
			slog.Error("failed to close Redis connection", "error", err)
		}
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return rdb, nil
}
