package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

type mockRow struct {
	scanFn func(dest ...interface{}) error
}

func (m mockRow) Scan(dest ...interface{}) error {
	if m.scanFn == nil {
		return nil
	}
	return m.scanFn(dest...)
}

type mockDB struct {
	execFn     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) pgx.Row
	closeFn    func()
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if m.execFn == nil {
		return pgconn.NewCommandTag(""), nil
	}
	return m.execFn(ctx, sql, args...)
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if m.queryRowFn == nil {
		return mockRow{}
	}
	return m.queryRowFn(ctx, sql, args...)
}

func (m *mockDB) Close() {
	if m.closeFn != nil {
		m.closeFn()
	}
}

type mockRedis struct {
	setFn    func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	existsFn func(ctx context.Context, keys ...string) *redis.IntCmd
	getFn    func(ctx context.Context, key string) *redis.StringCmd
	closeFn  func() error
}

func (m *mockRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if m.setFn == nil {
		return redis.NewStatusResult("OK", nil)
	}
	return m.setFn(ctx, key, value, expiration)
}

func (m *mockRedis) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if m.existsFn == nil {
		return redis.NewIntResult(0, nil)
	}
	return m.existsFn(ctx, keys...)
}

func (m *mockRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	if m.getFn == nil {
		return redis.NewStringResult("", redis.Nil)
	}
	return m.getFn(ctx, key)
}

func (m *mockRedis) Close() error {
	if m.closeFn == nil {
		return nil
	}
	return m.closeFn()
}
