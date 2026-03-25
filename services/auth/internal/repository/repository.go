package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

func New(db *pgxpool.Pool, redis *redis.Client) *Repository {
	if db == nil || redis == nil {
		panic("nil db or redis")
	}

	return &Repository{DB: db, Redis: redis}
}

func (r *Repository) Close(ctx context.Context) error {
	var errs []error
	if r.DB != nil {
		r.DB.Close()
	}
	if r.Redis != nil {
		if err := r.Redis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
