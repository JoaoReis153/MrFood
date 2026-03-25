package app

import (
	pb "MrFood/services/auth/internal/api/grpc/pb"
	"MrFood/services/auth/internal/auth"
	"MrFood/services/auth/internal/service"
	models "MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedAuthServiceServer
	authService    *service.Service
	authentication *auth.Auth
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *server) RegisterProcess(ctx context.Context, req *pb.Register) (*pb.RegisterResponse, error) {
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
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RegisterResponse{
		Id:       newUser.ID,
		Username: newUser.Username,
	}, nil

}

func (s *server) LoginProcess(ctx context.Context, req *pb.Login) (*pb.LoginResponse, error) {
	slog.Debug("login process", "email", req.Email, "password", req.Password)

	user, err := s.authService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid password")
	}

	token, err := s.authentication.CreateToken(string(user.ID), user.Username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// TODO: store redis session
	return &pb.LoginResponse{
		Token: token,
		User: &pb.User{
			Id:       user.ID,
			Username: user.Username,
		},
	}, nil
}

func (app *App) RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	authInstance, err := auth.New(context.Background())
	if err != nil {
		slog.Error("failed to create auth", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterAuthServiceServer(s, &server{
		authService:    app.Service,
		authentication: authInstance,
	})

	slog.Info("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
}
