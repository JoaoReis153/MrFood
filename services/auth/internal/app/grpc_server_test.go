package app

import (
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/auth"
	"MrFood/services/auth/internal/service"
	models "MrFood/services/auth/pkg"
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authSvcMock struct {
	store func(context.Context, *models.User) (*models.User, error)
	get   func(context.Context, string) (*models.User, error)
}

func (m *authSvcMock) StoreUser(ctx context.Context, user *models.User) (*models.User, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store(ctx, user)
}

func (m *authSvcMock) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.get == nil {
		return nil, nil
	}
	return m.get(ctx, email)
}

type jwtSvcMock struct {
	revokeAll func(context.Context, string) error
	genPair   func(context.Context, string, string) (*auth.TokenPair, error)
	refresh   func(context.Context, string) (*auth.TokenPair, error)
	validate  func(context.Context, string) (*auth.Claims, error)
	revokeOne func(context.Context, string) error
}

func (m *jwtSvcMock) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if m.revokeAll == nil {
		return nil
	}
	return m.revokeAll(ctx, userID)
}

func (m *jwtSvcMock) GenerateTokenPair(ctx context.Context, userID, username string) (*auth.TokenPair, error) {
	if m.genPair == nil {
		return nil, nil
	}
	return m.genPair(ctx, userID, username)
}

func (m *jwtSvcMock) RefreshTokens(ctx context.Context, tokenStr string) (*auth.TokenPair, error) {
	if m.refresh == nil {
		return nil, nil
	}
	return m.refresh(ctx, tokenStr)
}

func (m *jwtSvcMock) ValidateAccessToken(ctx context.Context, tokenString string) (*auth.Claims, error) {
	if m.validate == nil {
		return nil, nil
	}
	return m.validate(ctx, tokenString)
}

func (m *jwtSvcMock) RevokeAccessToken(ctx context.Context, tokenString string) error {
	if m.revokeOne == nil {
		return nil
	}
	return m.revokeOne(ctx, tokenString)
}

func TestServer_PingPong(t *testing.T) {
	s := &Server{}
	resp, err := s.PingPong(context.Background(), &pb.Ping{Id: 10})
	if err != nil || resp.Id != 1 {
		t.Fatalf("unexpected ping response: resp=%+v err=%v", resp, err)
	}
}

func TestServer_RegisterProcess(t *testing.T) {
	t.Run("hash error", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{}, jwtService: &jwtSvcMock{}}
		_, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "john@mail.com", Password: string(make([]byte, 73))})
		if err == nil {
			t.Fatal("expected hash error")
		}
	})

	t.Run("store fail maps to internal", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{store: func(context.Context, *models.User) (*models.User, error) {
			return nil, errors.New("db")
		}}, jwtService: &jwtSvcMock{}}
		_, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "john@mail.com", Password: "secret"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected internal, got %v", status.Code(err))
		}
	})

	t.Run("store duplicate maps to already exists", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{store: func(context.Context, *models.User) (*models.User, error) {
			return nil, service.ErrDuplicateUser
		}}, jwtService: &jwtSvcMock{}}
		_, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "john@mail.com", Password: "secret"})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("expected already exists, got %v", status.Code(err))
		}
	})

	t.Run("success", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{store: func(_ context.Context, u *models.User) (*models.User, error) {
			if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("secret")) != nil {
				t.Fatal("password not hashed")
			}
			return &models.User{ID: 1, Username: "john"}, nil
		}}, jwtService: &jwtSvcMock{}}
		resp, err := s.RegisterProcess(context.Background(), &pb.Register{Username: "john", Email: "john@mail.com", Password: "secret"})
		if err != nil || resp.Id != 1 || resp.Username != "john" {
			t.Fatalf("unexpected register result: resp=%+v err=%v", resp, err)
		}
	})
}

func TestServer_LoginProcess(t *testing.T) {
	h, _ := bcrypt.GenerateFromPassword([]byte("ok"), bcrypt.DefaultCost)

	t.Run("not found", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{get: func(context.Context, string) (*models.User, error) {
			return nil, errors.New("missing")
		}}, jwtService: &jwtSvcMock{}}
		_, err := s.LoginProcess(context.Background(), &pb.Login{Email: "john@mail.com", Password: "ok"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected not found, got %v", status.Code(err))
		}
	})

	t.Run("bad password", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{get: func(context.Context, string) (*models.User, error) {
			return &models.User{ID: 1, Username: "john", Password: string(h)}, nil
		}}, jwtService: &jwtSvcMock{}}
		_, err := s.LoginProcess(context.Background(), &pb.Login{Email: "john@mail.com", Password: "nope"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("jwt revoke fail", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{get: func(context.Context, string) (*models.User, error) {
			return &models.User{ID: 1, Username: "john", Password: string(h)}, nil
		}}, jwtService: &jwtSvcMock{revokeAll: func(context.Context, string) error { return errors.New("fail") }}}
		_, err := s.LoginProcess(context.Background(), &pb.Login{Email: "john@mail.com", Password: "ok"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected internal, got %v", status.Code(err))
		}
	})

	t.Run("success", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{get: func(context.Context, string) (*models.User, error) {
			return &models.User{ID: 1, Username: "john", Password: string(h)}, nil
		}}, jwtService: &jwtSvcMock{
			revokeAll: func(context.Context, string) error { return nil },
			genPair: func(context.Context, string, string) (*auth.TokenPair, error) {
				return &auth.TokenPair{AccessToken: "a", RefreshToken: "r"}, nil
			},
		}}
		resp, err := s.LoginProcess(context.Background(), &pb.Login{Email: "john@mail.com", Password: "ok"})
		if err != nil || resp.AccessToken != "a" || resp.RefreshToken != "r" {
			t.Fatalf("unexpected login result: resp=%+v err=%v", resp, err)
		}
	})
}

func TestServer_RefreshAndLogout(t *testing.T) {
	t.Run("refresh fail", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{}, jwtService: &jwtSvcMock{refresh: func(context.Context, string) (*auth.TokenPair, error) {
			return nil, errors.New("bad")
		}}}
		_, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "x"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("refresh success", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{}, jwtService: &jwtSvcMock{refresh: func(context.Context, string) (*auth.TokenPair, error) {
			return &auth.TokenPair{AccessToken: "a", RefreshToken: "r"}, nil
		}}}
		resp, err := s.RefreshTokenProcess(context.Background(), &pb.RefreshRequest{RefreshToken: "x"})
		if err != nil || resp.Token != "a" || resp.RefreshToken != "r" {
			t.Fatalf("unexpected refresh result: resp=%+v err=%v", resp, err)
		}
	})

	t.Run("logout validate fail", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{}, jwtService: &jwtSvcMock{validate: func(context.Context, string) (*auth.Claims, error) {
			return nil, errors.New("bad")
		}}}
		_, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: "x"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("logout success", func(t *testing.T) {
		s := &Server{authService: &authSvcMock{}, jwtService: &jwtSvcMock{
			validate:  func(context.Context, string) (*auth.Claims, error) { return &auth.Claims{UserID: "u1"}, nil },
			revokeOne: func(context.Context, string) error { return nil },
			revokeAll: func(context.Context, string) error { return nil },
		}}
		resp, err := s.LogoutProcess(context.Background(), &pb.LogoutRequest{Token: "x"})
		if err != nil || resp == nil {
			t.Fatalf("unexpected logout result: resp=%+v err=%v", resp, err)
		}
	})
}
