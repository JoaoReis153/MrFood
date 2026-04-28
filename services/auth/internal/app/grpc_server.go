package app

import (
	"MrFood/services/auth/config"
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/auth"
	"MrFood/services/auth/internal/service"
	models "MrFood/services/auth/pkg"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	authService         authService
	jwtService          jwtService
	notificationService notificationService
}

type authService interface {
	StoreUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}

type notificationService interface {
	SendRegistrationEmail(ctx context.Context, email, username string) error
}

type notificationClient struct {
	conn grpc.ClientConnInterface
}

const notificationSendRegistrationEmailMethod = "/notification.AuthToNotificationService/SendRegistrationEmail"

func newNotificationClient(target string) (*notificationClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial notification grpc: %w", err)
	}

	return &notificationClient{conn: conn}, conn, nil
}

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

type jwtService interface {
	RevokeAllUserTokens(ctx context.Context, userID string) error
	GenerateTokenPair(ctx context.Context, userID, username, email string) (*auth.TokenPair, error)
	RefreshTokens(ctx context.Context, tokenStr string) (*auth.TokenPair, error)
	ValidateAccessToken(ctx context.Context, tokenString string) (*auth.Claims, error)
	RevokeAccessToken(ctx context.Context, tokenString string) error
}

func (s *Server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *Server) RegisterProcess(ctx context.Context, req *pb.Register) (*pb.RegisterResponse, error) {
	slog.Info("registering user", "username", req.Username)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("hash password failed", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Email:    req.Email,
		Username: req.Username,
		Password: string(hashedPassword),
	}

	newUser, err := s.authService.StoreUser(ctx, user)
	if err != nil {
		slog.Error("failed to store user", "error", err)
		if errors.Is(err, service.ErrDuplicateUser) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.notificationService != nil {
		if err := s.notificationService.SendRegistrationEmail(ctx, req.GetEmail(), req.GetUsername()); err != nil {
			slog.Warn("failed to send registration email", "user_id", newUser.ID, "email", req.GetEmail(), "error", err)
		}
	}

	return &pb.RegisterResponse{
		Id:       newUser.ID,
		Username: newUser.Username,
	}, nil

}

func (s *Server) LoginProcess(ctx context.Context, req *pb.Login) (*pb.LoginResponse, error) {
	user, err := s.authService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		slog.Error("failed to get user", "error", err)
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if user == nil {
		slog.Error("user not found", "email", req.Email)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	userID := strconv.FormatInt(int64(user.ID), 10)

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		slog.Error("invalid password", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid password")
	}

	if err := s.jwtService.RevokeAllUserTokens(ctx, userID); err != nil {
		slog.Error("failed to revoke all user tokens", "error", err)
		return nil, status.Error(codes.Internal, "failed to revoke all user tokens")
	}

	tokenPair, err := s.jwtService.GenerateTokenPair(ctx, userID, user.Username, user.Email)
	if err != nil {
		slog.Error("failed to generate token pair", "error", err)
		return nil, status.Error(codes.Internal, "failed to generate token pair")
	}

	return &pb.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: &pb.User{
			Id:       int64(user.ID),
			Username: user.Username,
		},
	}, nil
}

func (s *Server) RefreshTokenProcess(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	tokenPair, err := s.jwtService.RefreshTokens(ctx, req.RefreshToken)
	if err != nil {
		slog.Error("failed to refresh token", "error", err)
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &pb.RefreshResponse{
		Token:        tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

func (s *Server) LogoutProcess(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	claims, err := s.jwtService.ValidateAccessToken(ctx, req.Token)
	if err != nil {
		slog.Error("failed to validate access token", "error", err)
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	if err := s.jwtService.RevokeAccessToken(ctx, req.Token); err != nil {
		slog.Error("failed to revoke access token", "error", err)
		return nil, status.Error(codes.Internal, "failed to revoke access token")
	}

	if err := s.jwtService.RevokeAllUserTokens(ctx, claims.UserID); err != nil {
		slog.Error("failed to revoke user tokens", "error", err)
		return nil, status.Error(codes.Internal, "failed to revoke user tokens")
	}

	return &pb.LogoutResponse{}, nil
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed", "error", err)
		return fmt.Errorf("listen: %w", err)
	}

	jwtServiceInstance := auth.NewJWTService(&cfg.JWT, app.Repo)
	notificationClient := &notificationClient{conn: app.notificationConn}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterAuthServiceServer(s, &Server{
		authService:         app.Service,
		jwtService:          jwtServiceInstance,
		notificationService: notificationClient,
	})
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "auth")

	slog.Info("gRPC server listening", "port", cfg.Server.Port)

	g, ctx := errgroup.WithContext(ctx)

	// start the gRPC server
	g.Go(func() error {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	})

	// listen for context cancellation to shut down the server
	g.Go(func() error {
		<-ctx.Done() // Wait for the context to be canceled
		slog.Info("shutting down gRPC server...")
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}
