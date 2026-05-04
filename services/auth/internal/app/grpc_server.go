package app

import (
	"MrFood/services/auth/config"
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/keycloak"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type keycloakClient interface {
	Login(ctx context.Context, email, password string) (*keycloak.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*keycloak.TokenResponse, error)
	CreateUser(ctx context.Context, username, email, password string) (string, error)
	RevokeAllUserSessions(ctx context.Context, userID string) error
}

type Server struct {
	pb.UnimplementedAuthServiceServer
	kc                  keycloakClient
	notificationService notificationService
}

type notificationService interface {
	SendRegistrationEmail(ctx context.Context, email, username string) error
}

type notificationClient struct {
	conn grpc.ClientConnInterface
}

const notificationSendRegistrationEmailMethod = "/notification.AuthToNotificationService/SendRegistrationEmail"

func (c *notificationClient) SendRegistrationEmail(ctx context.Context, email, username string) error {
	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &pb.SendRegistrationEmailRequest{
		Email:    email,
		Username: username,
	}

	resp := new(pb.SendRegistrationEmailResponse)
	return c.conn.Invoke(
		notifyCtx,
		notificationSendRegistrationEmailMethod,
		req,
		resp,
	)
}

// ──────────────────────────────────────────────────────────────────────────────
// RPC handlers
// ──────────────────────────────────────────────────────────────────────────────

func (s *Server) PingPong(_ context.Context, _ *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{Id: 1}, nil
}

// RegisterProcess creates a new user in Keycloak.
func (s *Server) RegisterProcess(ctx context.Context, req *pb.Register) (*pb.RegisterResponse, error) {
	slog.Info("registering user", "username", req.Username)

	kcUserID, err := s.kc.CreateUser(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		slog.Error("failed to create keycloak user", "error", err)
		if errors.Is(err, keycloak.ErrUserAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	slog.Info("user created in keycloak", "kc_id", kcUserID)

	if s.notificationService != nil {
		if err := s.notificationService.SendRegistrationEmail(ctx, req.Email, req.Username); err != nil {
			slog.Warn("failed to send registration email", "email", req.Email, "error", err)
		}
	}

	return &pb.RegisterResponse{
		Id:       uuidToInt64(kcUserID),
		Username: req.Username,
	}, nil
}

func (s *Server) LoginProcess(ctx context.Context, req *pb.Login) (*pb.LoginResponse, error) {
	slog.Info("login attempt", "email", req.Email)

	tokenResp, err := s.kc.Login(ctx, req.Email, req.Password)
	if err != nil {
		slog.Error("keycloak login failed", "error", err)
		if errors.Is(err, keycloak.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	claims, err := parseJWTPayload(tokenResp.AccessToken)
	if err != nil {
		slog.Error("failed to parse keycloak access token", "error", err)
		return nil, status.Error(codes.Internal, "failed to parse token claims")
	}

	sub, _ := claims["sub"].(string)
	username, _ := claims["preferred_username"].(string)

	return &pb.LoginResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		User: &pb.User{
			Id:       uuidToInt64(sub),
			Username: username,
		},
	}, nil
}

func (s *Server) RefreshTokenProcess(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	tokenResp, err := s.kc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		slog.Error("keycloak refresh failed", "error", err)
		if errors.Is(err, keycloak.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "refresh token invalid or expired")
		}
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &pb.RefreshResponse{
		Token:        tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}, nil
}

func (s *Server) LogoutProcess(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	claims, err := parseJWTPayload(req.Token)
	if err != nil {
		slog.Error("logout: failed to parse token", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, status.Error(codes.Unauthenticated, "token missing sub claim")
	}

	if err := s.kc.RevokeAllUserSessions(ctx, sub); err != nil {
		slog.Error("failed to revoke keycloak sessions", "sub", sub, "error", err)
		return nil, status.Error(codes.Internal, "failed to revoke sessions")
	}

	slog.Info("user logged out", "sub", sub)
	return &pb.LogoutResponse{}, nil
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed to listen", "error", err)
		return fmt.Errorf("listen: %w", err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterAuthServiceServer(s, &Server{
		kc:                  app.KC,
		notificationService: &notificationClient{conn: app.notificationConn},
	})

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered", "service", "auth")

	slog.Info("gRPC server listening", "port", cfg.Server.Port)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		slog.Info("shutting down gRPC server")
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}

func parseJWTPayload(tokenString string) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}
	return claims, nil
}

func uuidToInt64(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() &^ (uint64(1) << 63)) // clear high bit → always non-negative
}
