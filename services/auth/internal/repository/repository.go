package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type dbClient interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

type redisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Close() error
}

type Repository struct {
	DB    dbClient
	Redis redisClient
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
