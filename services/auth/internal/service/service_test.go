package service

import (
	"MrFood/services/auth/pkg"
	"context"
	"errors"
	"strings"
	"testing"
)

type mockUserStore struct {
	createUserFn func(ctx context.Context, username, password, email string) (int64, string, error)
	getUserFn    func(ctx context.Context, email string) (*pkg.User, error)
}

func (m *mockUserStore) CreateUser(ctx context.Context, username, password, email string) (int64, string, error) {
	if m.createUserFn == nil {
		return 0, "", nil
	}
	return m.createUserFn(ctx, username, password, email)
}

func (m *mockUserStore) GetUser(ctx context.Context, email string) (*pkg.User, error) {
	if m.getUserFn == nil {
		return nil, nil
	}
	return m.getUserFn(ctx, email)
}

func TestStoreUser(t *testing.T) {
	t.Run("validation fails", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{}}
		_, err := svc.StoreUser(context.Background(), &pkg.User{Username: "ab", Email: "bad-email", Password: ""})
		if err == nil || !strings.Contains(err.Error(), "user validation failed") {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("repo create fails", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{
			createUserFn: func(ctx context.Context, username, password, email string) (int64, string, error) {
				return 0, "", errors.New("db down")
			},
		}}

		_, err := svc.StoreUser(context.Background(), &pkg.User{Username: "john", Email: "john@mail.com", Password: "hashed"})
		if err == nil || !strings.Contains(err.Error(), "failed to create user") {
			t.Fatalf("expected create error, got %v", err)
		}
	})

	t.Run("success returns id and username only", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{
			createUserFn: func(ctx context.Context, username, password, email string) (int64, string, error) {
				return 42, "john", nil
			},
		}}

		user, err := svc.StoreUser(context.Background(), &pkg.User{Username: "john", Email: "john@mail.com", Password: "hashed"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 42 || user.Username != "john" || user.Email != "" || user.Password != "" {
			t.Fatalf("unexpected user payload: %+v", user)
		}
	})
}

func TestGetUserByEmail(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{
			getUserFn: func(ctx context.Context, email string) (*pkg.User, error) {
				return nil, errors.New("db error")
			},
		}}

		_, err := svc.GetUserByEmail(context.Background(), "john@mail.com")
		if err == nil || !strings.Contains(err.Error(), "failed to fetch user") {
			t.Fatalf("expected fetch error, got %v", err)
		}
	})

	t.Run("nil user", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{}}
		_, err := svc.GetUserByEmail(context.Background(), "john@mail.com")
		if err == nil || !strings.Contains(err.Error(), "user not found") {
			t.Fatalf("expected user not found error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := &Service{repo: &mockUserStore{
			getUserFn: func(ctx context.Context, email string) (*pkg.User, error) {
				return &pkg.User{ID: 11, Username: "john", Email: email, Password: "hash"}, nil
			},
		}}

		user, err := svc.GetUserByEmail(context.Background(), "john@mail.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != 11 || user.Username != "john" || user.Email != "john@mail.com" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})
}
