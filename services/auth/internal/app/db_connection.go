package app

import (
	"MrFood/services/auth/config"
	"context"
	"fmt"
	"log/slog"
	"net/url" // ← ADD THIS

	"github.com/jackc/pgx/v5/pgxpool"
)

func (app *App) ConnectDb() error {
	cfg := config.Get(context.Background())

	// DEBUG: Verify config loads
	slog.Info("db config loaded",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"user", cfg.DB.User,
		"db", cfg.DB.Name,
		"pass_len", len(cfg.DB.Password),
	)

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DB.User,
		url.QueryEscape(cfg.DB.Password),
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	slog.Info("db connecting", "connstr", connStr) // ← See exact string

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		slog.Error("db pool failed", "error", err, "connstr", connStr)
		return fmt.Errorf("db connect: %w", err)
	}

	// Ping to verify
	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("db ping failed", "error", err)
		pool.Close()
		return fmt.Errorf("db ping: %w", err)
	}

	app.DB = pool
	slog.Info("db connected successfully")
	return nil
}
