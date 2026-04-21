package repository

import (
	"MrFood/services/payment/config"
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Connections struct {
	DB *pgxpool.Pool
}

func NewConnections(ctx context.Context, cfg *config.Config) (*Connections, error) {
	dbPool, err := NewDBPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Connections{
		DB: dbPool,
	}, nil
}

func (c *Connections) Close() {
	if c == nil {
		return
	}
	if c.DB != nil {
		c.DB.Close()
	}
}

func NewDBPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DB.User,
		url.QueryEscape(cfg.DB.Password),
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	poolCfg.MaxConns = cfg.DB.MaxConns
	poolCfg.MinConns = cfg.DB.MinConns
	poolCfg.MaxConnLifetime = cfg.DB.MaxConnLifetime
	poolCfg.HealthCheckPeriod = cfg.DB.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return pool, nil
}
