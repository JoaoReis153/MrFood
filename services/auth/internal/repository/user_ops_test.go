package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCreateUser(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r := &Repository{DB: &mockDB{}, Redis: &mockRedis{}}
		_, _, err := r.CreateUser(ctx, "john", "pwd", "john@mail.com")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{
				queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
					return mockRow{scanFn: func(dest ...interface{}) error {
						return errors.New("insert failed")
					}}
				},
			},
			Redis: &mockRedis{},
		}

		_, _, err := r.CreateUser(context.Background(), "john", "pwd", "john@mail.com")
		if err == nil || !strings.Contains(err.Error(), "failed to create user") {
			t.Fatalf("expected create user error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{
				queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
					return mockRow{scanFn: func(dest ...interface{}) error {
						id := dest[0].(*int64)
						name := dest[1].(*string)
						*id = 7
						*name = "john"
						return nil
					}}
				},
			},
			Redis: &mockRedis{},
		}

		id, username, err := r.CreateUser(context.Background(), "john", "pwd", "john@mail.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != 7 || username != "john" {
			t.Fatalf("unexpected return values: id=%d username=%s", id, username)
		}
	})
}

func TestGetUser(t *testing.T) {
	t.Run("db error", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{
				queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
					return mockRow{scanFn: func(dest ...interface{}) error {
						return errors.New("query failed")
					}}
				},
			},
			Redis: &mockRedis{},
		}

		_, err := r.GetUser(context.Background(), "john@mail.com")
		if err == nil || !strings.Contains(err.Error(), "failed to get user") {
			t.Fatalf("expected get user error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		r := &Repository{
			DB: &mockDB{
				queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
					return mockRow{scanFn: func(dest ...interface{}) error {
						id := dest[0].(*int64)
						username := dest[1].(*string)
						password := dest[2].(*string)
						*id = 9
						*username = "john"
						*password = "hashed"
						return nil
					}}
				},
			},
			Redis: &mockRedis{},
		}

		user, err := r.GetUser(context.Background(), "john@mail.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 9 || user.Username != "john" || user.Email != "john@mail.com" || user.Password != "hashed" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})
}
