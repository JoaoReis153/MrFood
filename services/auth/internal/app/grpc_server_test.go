package app

import (
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/keycloak"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	jwtlib "github.com/golang-jwt/jwt/v5"
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
	kcToken := fakeJWT(sub, username) // Keycloak-issued token (what the mock returns)

	t.Run("invalid credentials maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{login: func(context.Context, string, string) (*keycloak.TokenResponse, error) {
			return nil, keycloak.ErrInvalidCredentials
		}}, jwtSecret: []byte("test-secret")}
		_, err := s.LoginProcess(context.Background(), &pb.Login{Email: "j@mail.com", Password: "bad"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("success mints HS256 access token with correct claims", func(t *testing.T) {
		s := &Server{kc: &kcMock{login: func(context.Context, string, string) (*keycloak.TokenResponse, error) {
			return &keycloak.TokenResponse{AccessToken: kcToken, RefreshToken: "rt"}, nil
		}}, jwtSecret: []byte("test-secret")}
		resp, err := s.LoginProcess(context.Background(), &pb.Login{Email: "j@mail.com", Password: "ok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Access token must be a fresh HS256 JWT (not the raw Keycloak token)
		if resp.AccessToken == kcToken {
			t.Fatal("expected a freshly minted token, got the raw Keycloak token")
		}
		if !strings.HasPrefix(resp.AccessToken, "ey") || strings.Count(resp.AccessToken, ".") != 2 {
			t.Fatalf("access token does not look like a JWT: %s", resp.AccessToken)
		}
		if resp.RefreshToken != "rt" {
			t.Fatalf("refresh token mismatch: got %s", resp.RefreshToken)
		}
		// Verify the minted token contains the right claims
		var claims appClaims
		if _, err := jwtlib.ParseWithClaims(resp.AccessToken, &claims, func(t *jwtlib.Token) (interface{}, error) {
			return []byte("test-secret"), nil
		}); err != nil {
			t.Fatalf("failed to parse minted token: %v", err)
		}
		if claims.Subject != sub {
			t.Fatalf("sub mismatch: got %s, want %s", claims.Subject, sub)
		}
		if claims.UserID != sub {
			t.Fatalf("user_id mismatch: got %s, want %s", claims.UserID, sub)
		}
		if claims.Username != username {
			t.Fatalf("username mismatch: got %s, want %s", claims.Username, username)
		}
		if resp.User.Id != uuidToInt64(sub) {
			t.Fatalf("user id mismatch: got %d, want %d", resp.User.Id, uuidToInt64(sub))
		}
	})
}

func TestServer_RefreshTokenProcess(t *testing.T) {
	const (
		sub      = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		username = "refreshuser"
	)

	t.Run("refresh failure maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{refresh: func(context.Context, string) (*keycloak.TokenResponse, error) {
			return nil, errors.New("expired")
		}}, jwtSecret: []byte("test-secret")}
		_, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "old"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("success mints new HS256 access token", func(t *testing.T) {
		// The mock must return a parseable Keycloak JWT (with sub and preferred_username)
		newKCToken := fakeJWT(sub, username)
		s := &Server{kc: &kcMock{refresh: func(context.Context, string) (*keycloak.TokenResponse, error) {
			return &keycloak.TokenResponse{AccessToken: newKCToken, RefreshToken: "new-rt"}, nil
		}}, jwtSecret: []byte("test-secret")}
		resp, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "old"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Token == newKCToken {
			t.Fatal("expected a freshly minted token, got the raw Keycloak token")
		}
		if resp.RefreshToken != "new-rt" {
			t.Fatalf("refresh token mismatch: got %s", resp.RefreshToken)
		}
		// Verify minted token claims
		var claims appClaims
		if _, err := jwtlib.ParseWithClaims(resp.Token, &claims, func(t *jwtlib.Token) (interface{}, error) {
			return []byte("test-secret"), nil
		}); err != nil {
			t.Fatalf("failed to parse minted refresh token: %v", err)
		}
		if claims.UserID != sub || claims.Username != username {
			t.Fatalf("claim mismatch: user_id=%s username=%s", claims.UserID, claims.Username)
		}
	})
}

func TestServer_LogoutProcess(t *testing.T) {
	const sub = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	t.Run("malformed token maps to Unauthenticated", func(t *testing.T) {
		s := &Server{kc: &kcMock{}, jwtSecret: []byte("test-secret")}
		_, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: "not-a-jwt"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("revoke failure maps to Internal", func(t *testing.T) {
		token := fakeJWT(sub, "user")
		s := &Server{kc: &kcMock{revokeSess: func(context.Context, string) error {
			return errors.New("admin api down")
		}}, jwtSecret: []byte("test-secret")}
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
		}}, jwtSecret: []byte("test-secret")}
		resp, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: token})
		if err != nil || resp == nil {
			t.Fatalf("unexpected logout result: resp=%+v err=%v", resp, err)
		}
		if capturedSub != sub {
			t.Fatalf("wrong sub passed to revoke: got %s, want %s", capturedSub, sub)
		}
	})
}
