package app

import (
	"MrFood/services/sponsor/config"
	pb "MrFood/services/sponsor/internal/api/grpc/pb"
	"MrFood/services/sponsor/internal/service"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	models "MrFood/services/sponsor/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedSponsorServiceServer
	sponsorService *service.Service
}

type UserInfo struct {
	UserID   int32
	Username string
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *server) GetRestaurantSponsorship(ctx context.Context, req *pb.GetRestaurantSponsorshipRequest) (*pb.SponsorshipResponse, error) {

	slog.Info("get restaurant sponsorship: ", req)

	response, err := s.sponsorService.GetRestaurantSponsorship(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &pb.SponsorshipResponse{
		Id:    int32(response.ID),
		Tier:  int32(response.Tier),
		Until: timestamppb.New(response.Until),
	}, nil
}

func (s *server) Sponsor(ctx context.Context, req *pb.SponsorshipRequest) (*pb.SponsorshipResponse, error) {
	if req.Tier < 0 || req.Tier > 4 {
		return nil, status.Error(codes.InvalidArgument, "Tier must be between 0 and 4")
	}

	user, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	slog.Info("USER: ", user.Username)

	sponsorship := &models.Sponsorship{
		ID:         int(req.Id),
		Tier:       int(req.Tier),
		Until:      time.Now().AddDate(0, 1, 0),
		Categories: []string{},
	}

	response, err := s.sponsorService.Sponsor(ctx, sponsorship, int(user.UserID))
	if err != nil {
		return nil, err
	}

	slog.Info("ADDED TO DATABASE: ", response.ID, response.Tier, response.Until)

	return &pb.SponsorshipResponse{
		Id:    int32(response.ID),
		Tier:  int32(response.Tier),
		Until: timestamppb.New(response.Until),
	}, nil
}

func ExtractUserFromContext(ctx context.Context) (*UserInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return nil, errors.New("no auth header")
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")

	claims := &Claims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims)

	if err != nil {
		slog.Error("failed to parse token", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	userID, err := parseInt32(claims.UserID)

	if err != nil {
		slog.Error("failed to parse user id", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid user id in token")
	}

	userInfo := &UserInfo{
		UserID:   userID,
		Username: claims.Username,
	}

	slog.Info("USER INFO",
		"user_id", claims.UserID,
		"username", claims.Username,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return userInfo, nil
}

func parseInt32(value string) (int32, error) {
	v, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, errors.New("out of int32 range")
	}
	return int32(v), nil
}

func (app *App) RunServer() {
	cfg := config.Get(context.Background())
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	srv := &server{
		sponsorService: app.Service,
	}

	pb.RegisterSponsorServiceServer(s, srv)

	slog.Info("server running", "addr", addr)
	if err := s.Serve(lis); err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
}

func NewClient(address string) (pb.RestaurantToSponsorServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewRestaurantToSponsorServiceClient(conn), conn, nil
}
