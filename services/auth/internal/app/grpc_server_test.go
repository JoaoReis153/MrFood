package app

import (
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/keycloak"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ──────────────────────────────────────────────────────────────────────────────
// Mock keycloakClient
// ──────────────────────────────────────────────────────────────────────────────

type kcMock struct {
	login      func(context.Context, string, string) (*keycloak.TokenResponse, error)
	refresh    func(context.Context, string) (*keycloak.TokenResponse, error)
	createUser func(context.Context, string, string, string) (string, error)
	revokeSess func(context.Context, string) error
}

func (m *kcMock) Login(ctx context.Context, email, password string) (*keycloak.TokenResponse, error) {
	if m.login == nil {
		return nil, nil
	}
	return m.login(ctx, email, password)
}

func (m *kcMock) RefreshToken(ctx context.Context, rt string) (*keycloak.TokenResponse, error) {
	if m.refresh == nil {
		return nil, nil
	}
	return m.refresh(ctx, rt)
}

func (m *kcMock) CreateUser(ctx context.Context, username, email, password string) (string, error) {
	if m.createUser == nil {
		return "fake-uuid", nil
	}
	return m.createUser(ctx, username, email, password)
}

func (m *kcMock) RevokeAllUserSessions(ctx context.Context, userID string) error {
	if m.revokeSess == nil {
		return nil
	}
	return m.revokeSess(ctx, userID)
}

// fakeJWT builds a minimal unsigned JWT (header.payload.sig) for test purposes.
func fakeJWT(sub, username string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf(`{"sub":%q,"preferred_username":%q}`, sub, username)),
	)
	return header + "." + payload + ".fakesig"
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestServer_PingPong(t *testing.T) {
	s := &Server{kc: &kcMock{}}
	resp, err := s.PingPong(context.Background(), &pb.Ping{Id: 10})
	if err != nil || resp.Id != 1 {
		t.Fatalf("unexpected ping response: resp=%+v err=%v", resp, err)
	}
}

func TestServer_RegisterProcess(t *testing.T) {
	t.Run("keycloak error maps to internal", func(t *testing.T) {
		s := &Server{kc: &kcMock{createUser: func(context.Context, string, string, string) (string, error) {
			return "", errors.New("kc down")
		}}}
		_, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "j@mail.com", Password: "secret"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected internal, got %v", status.Code(err))
		}
	})

	t.Run("duplicate user maps to AlreadyExists", func(t *testing.T) {
		s := &Server{kc: &kcMock{createUser: func(context.Context, string, string, string) (string, error) {
			return "", keycloak.ErrUserAlreadyExists
		}}}
		_, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "j@mail.com", Password: "secret"})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("expected AlreadyExists, got %v", status.Code(err))
		}
	})

	t.Run("success returns deterministic id and username", func(t *testing.T) {
		const fakeUUID = "11111111-1111-1111-1111-111111111111"
		s := &Server{kc: &kcMock{createUser: func(context.Context, string, string, string) (string, error) {
			return fakeUUID, nil
		}}}
		resp, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "j@mail.com", Password: "secret"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Id != uuidToInt64(fakeUUID) {
			t.Fatalf("id mismatch: got %d, want %d", resp.Id, uuidToInt64(fakeUUID))
		}
		if resp.Username != "john" {
			t.Fatalf("username mismatch: got %s", resp.Username)
		}
	})
}

func TestServer_LoginProcess(t *testing.T) {
	const (
		sub      = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		username = "johndoe"
	)
	token := fakeJWT(sub, username)

	t.Run("invalid credentials maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{login: func(context.Context, string, string) (*keycloak.TokenResponse, error) {
			return nil, keycloak.ErrInvalidCredentials
		}}}
		_, err := s.LoginProcess(context.Background(), &pb.Login{Email: "j@mail.com", Password: "bad"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("success returns tokens and user", func(t *testing.T) {
		s := &Server{kc: &kcMock{login: func(context.Context, string, string) (*keycloak.TokenResponse, error) {
			return &keycloak.TokenResponse{AccessToken: token, RefreshToken: "rt"}, nil
		}}}
		resp, err := s.LoginProcess(context.Background(), &pb.Login{Email: "j@mail.com", Password: "ok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.AccessToken != token || resp.RefreshToken != "rt" {
			t.Fatalf("token mismatch: %+v", resp)
		}
		if resp.User.Id != uuidToInt64(sub) {
			t.Fatalf("user id mismatch: got %d, want %d", resp.User.Id, uuidToInt64(sub))
		}
		if resp.User.Username != username {
			t.Fatalf("username mismatch: got %s", resp.User.Username)
		}
	})
}

func TestServer_RefreshTokenProcess(t *testing.T) {
	t.Run("refresh failure maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{refresh: func(context.Context, string) (*keycloak.TokenResponse, error) {
			return nil, errors.New("expired")
		}}}
		_, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "old"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("success returns new token pair", func(t *testing.T) {
		s := &Server{kc: &kcMock{refresh: func(context.Context, string) (*keycloak.TokenResponse, error) {
			return &keycloak.TokenResponse{AccessToken: "new-at", RefreshToken: "new-rt"}, nil
		}}}
		resp, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "old"})
		if err != nil || resp.Token != "new-at" || resp.RefreshToken != "new-rt" {
			t.Fatalf("unexpected refresh result: resp=%+v err=%v", resp, err)
		}
	})
}

func TestServer_LogoutProcess(t *testing.T) {
	const sub = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	t.Run("malformed token maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{}}
		_, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: "not-a-jwt"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("revoke failure maps to Internal", func(t *testing.T) {
		token := fakeJWT(sub, "user")
		s := &Server{kc: &kcMock{revokeSess: func(context.Context, string) error {
			return errors.New("admin api down")
		}}}
		_, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: token})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v", status.Code(err))
		}
	})

	t.Run("success", func(t *testing.T) {
		token := fakeJWT(sub, "user")
		var capturedSub string
		s := &Server{kc: &kcMock{revokeSess: func(_ context.Context, id string) error {
			capturedSub = id
			return nil
		}}}
		resp, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: token})
		if err != nil || resp == nil {
			t.Fatalf("unexpected logout result: resp=%+v err=%v", resp, err)
		}
		if capturedSub != sub {
			t.Fatalf("wrong sub passed to revoke: got %s, want %s", capturedSub, sub)
		}
	})
}
