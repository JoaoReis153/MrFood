package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

func TestStoreRefreshToken(t *testing.T) {
	t.Run("db error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{execFn: func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag(""), errors.New("insert fail")
			}},
			Redis: &mockRedis{},
		}
		err := r.StoreRefreshToken(context.Background(), "t1", "u1", time.Now().Add(time.Hour))
		if err == nil || !strings.Contains(err.Error(), "failed to store refresh token") {
			t.Fatalf("expected store error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{}}
		if err := r.StoreRefreshToken(context.Background(), "t1", "u1", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetRefreshToken(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error { return pgx.ErrNoRows }}
			}},
			Redis: &mockRedis{},
		}
		data, err := r.GetRefreshToken(context.Background(), "x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data != nil {
			t.Fatalf("expected nil token data, got %+v", data)
		}
	})

	t.Run("db error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error { return errors.New("db fail") }}
			}},
			Redis: &mockRedis{},
		}
		_, err := r.GetRefreshToken(context.Background(), "x")
		if err == nil || !strings.Contains(err.Error(), "failed to get refresh token") {
			t.Fatalf("expected db error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "token-1"
					*(dest[1].(*string)) = "user-1"
					*(dest[2].(*time.Time)) = now.Add(time.Hour)
					*(dest[3].(*bool)) = false
					*(dest[4].(*time.Time)) = now
					return nil
				}}
			}},
			Redis: &mockRedis{},
		}
		data, err := r.GetRefreshToken(context.Background(), "token-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.TokenID != "token-1" || data.UserID != "user-1" {
			t.Fatalf("unexpected token data: %+v", data)
		}
	})
}

func TestBlacklistToken(t *testing.T) {
	t.Run("expired ttl no-op", func(t *testing.T) {
		called := false
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			called = true
			return redis.NewStatusResult("OK", nil)
		}}}
		if err := r.BlacklistToken(context.Background(), "x", time.Now().Add(-time.Second)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("expected redis set not to be called for expired token")
		}
	})

	t.Run("redis error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("", errors.New("redis down"))
		}}}
		err := r.BlacklistToken(context.Background(), "x", time.Now().Add(time.Minute))
		if err == nil || !strings.Contains(err.Error(), "failed to blacklist token") {
			t.Fatalf("expected blacklist error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{}}
		if err := r.BlacklistToken(context.Background(), "x", time.Now().Add(time.Minute)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestIsTokenBlacklisted(t *testing.T) {
	t.Run("redis error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			return redis.NewIntResult(0, errors.New("redis down"))
		}}}
		_, err := r.IsTokenBlacklisted(context.Background(), "x")
		if err == nil || !strings.Contains(err.Error(), "failed to check token blacklist") {
			t.Fatalf("expected blacklist check error, got %v", err)
		}
	})

	t.Run("true", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			return redis.NewIntResult(1, nil)
		}}}
		ok, err := r.IsTokenBlacklisted(context.Background(), "x")
		if err != nil || !ok {
			t.Fatalf("expected true,nil got %v,%v", ok, err)
		}
	})

	t.Run("false", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			return redis.NewIntResult(0, nil)
		}}}
		ok, err := r.IsTokenBlacklisted(context.Background(), "x")
		if err != nil || ok {
			t.Fatalf("expected false,nil got %v,%v", ok, err)
		}
	})
}

func TestGetUserTokenVersion(t *testing.T) {
	t.Run("cache hit", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{getFn: func(ctx context.Context, key string) *redis.StringCmd {
			return redis.NewStringResult("7", nil)
		}}}
		v, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err != nil || v != 7 {
			t.Fatalf("expected 7,nil got %d,%v", v, err)
		}
	})

	t.Run("redis get error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{getFn: func(ctx context.Context, key string) *redis.StringCmd {
			return redis.NewStringResult("", errors.New("redis read fail"))
		}}}
		_, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to get token version from cache") {
			t.Fatalf("expected cache error, got %v", err)
		}
	})

	t.Run("cache miss db error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error { return errors.New("db fail") }}
			}},
			Redis: &mockRedis{getFn: func(ctx context.Context, key string) *redis.StringCmd {
				return redis.NewStringResult("", redis.Nil)
			}},
		}
		_, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to get token version from db") {
			t.Fatalf("expected db error, got %v", err)
		}
	})

	t.Run("cache miss db no rows returns zero", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error { return pgx.ErrNoRows }}
			}},
			Redis: &mockRedis{getFn: func(ctx context.Context, key string) *redis.StringCmd {
				return redis.NewStringResult("", redis.Nil)
			}},
		}
		v, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err != nil || v != 0 {
			t.Fatalf("expected 0,nil got %d,%v", v, err)
		}
	})

	t.Run("cache miss set error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 4
					return nil
				}}
			}},
			Redis: &mockRedis{
				getFn: func(ctx context.Context, key string) *redis.StringCmd {
					return redis.NewStringResult("", redis.Nil)
				},
				setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
					return redis.NewStatusResult("", errors.New("redis set fail"))
				},
			},
		}
		_, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to set token version in redis") {
			t.Fatalf("expected set error, got %v", err)
		}
	})

	t.Run("cache miss success", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
				return mockRow{scanFn: func(dest ...interface{}) error {
					*(dest[0].(*int)) = 5
					return nil
				}}
			}},
			Redis: &mockRedis{getFn: func(ctx context.Context, key string) *redis.StringCmd {
				return redis.NewStringResult("", redis.Nil)
			}},
		}
		v, err := r.GetUserTokenVersion(context.Background(), "u1")
		if err != nil || v != 5 {
			t.Fatalf("expected 5,nil got %d,%v", v, err)
		}
	})
}

func TestIncrementUserTokenVersion(t *testing.T) {
	t.Run("db error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return mockRow{scanFn: func(dest ...interface{}) error { return errors.New("db fail") }}
		}}, Redis: &mockRedis{}}
		_, err := r.IncrementUserTokenVersion(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to increment token version") {
			t.Fatalf("expected increment error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		r := &Repository{DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int64)) = 3
				return nil
			}}
		}}, Redis: &mockRedis{}}
		v, err := r.IncrementUserTokenVersion(context.Background(), "u1")
		if err != nil || v != 3 {
			t.Fatalf("expected 3,nil got %d,%v", v, err)
		}
	})

	t.Run("redis set failure ignored", func(t *testing.T) {
		r := &Repository{DB: &mockDB{queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return mockRow{scanFn: func(dest ...interface{}) error {
				*(dest[0].(*int64)) = 4
				return nil
			}}
		}}, Redis: &mockRedis{setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("", errors.New("redis down"))
		}}}
		v, err := r.IncrementUserTokenVersion(context.Background(), "u1")
		if err != nil || v != 4 {
			t.Fatalf("expected 4,nil got %d,%v", v, err)
		}
	})
}

func TestRevokeRefreshTokenAndRevokeAllUserTokens(t *testing.T) {
	t.Run("revoke refresh token db error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{execFn: func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("db fail")
		}}, Redis: &mockRedis{}}
		err := r.RevokeRefreshToken(context.Background(), "t1")
		if err == nil || !strings.Contains(err.Error(), "failed to revoke refresh token") {
			t.Fatalf("expected revoke refresh error, got %v", err)
		}
	})

	t.Run("revoke refresh token success", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{}}
		if err := r.RevokeRefreshToken(context.Background(), "t1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("revoke all user tokens db error", func(t *testing.T) {
		r := &Repository{DB: &mockDB{execFn: func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("db fail")
		}}, Redis: &mockRedis{}}
		err := r.RevokeAllUserTokens(context.Background(), "u1")
		if err == nil || !strings.Contains(err.Error(), "failed to revoke all user tokens") {
			t.Fatalf("expected revoke all error, got %v", err)
		}
	})

	t.Run("revoke all user tokens success", func(t *testing.T) {
		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{}}
		if err := r.RevokeAllUserTokens(context.Background(), "u1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

