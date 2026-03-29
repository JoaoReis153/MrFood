package app

import (
	"MrFood/services/auth/config"
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/auth"
	models "MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	authService authService
	jwtService  jwtService
}

type authService interface {
	StoreUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}

type jwtService interface {
	RevokeAllUserTokens(ctx context.Context, userID string) error
	GenerateTokenPair(ctx context.Context, userID, username string) (*auth.TokenPair, error)
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
		return nil, status.Error(codes.Internal, err.Error())
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

	tokenPair, err := s.jwtService.GenerateTokenPair(ctx, userID, user.Username)
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

	s := grpc.NewServer()
	pb.RegisterAuthServiceServer(s, &Server{
		authService: app.Service,
		jwtService:  jwtServiceInstance,
	})

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
		s.GracefulStop()
		return nil
	})

	return g.Wait()
}
