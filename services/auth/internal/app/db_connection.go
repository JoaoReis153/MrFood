package app

import (
	"MrFood/services/auth/config"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (app *App) ConnectDb() error {
	cfg := config.Get()

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return err
	}

	app.DB = pool

	return nil
}
