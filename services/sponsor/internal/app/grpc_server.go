package app

import (
	"MrFood/services/sponsor/config"
	pb "MrFood/services/sponsor/internal/api/grpc/pb"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"hash/fnv"
	"strings"
	"time"

	models "MrFood/services/sponsor/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedSponsorServiceServer
	sponsorService SponsorService
}

type UserInfo struct {
	UserID   int64
	Email    string
	Username string
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

type SponsorService interface {
	GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error)
	Sponsor(ctx context.Context, s *models.Sponsorship, userID int64, email string) (*models.SponsorshipResponse, int32, error)
}

func (s *server) GetRestaurantSponsorship(ctx context.Context, req *pb.GetRestaurantSponsorshipRequest) (*pb.SponsorshipResponse, error) {

	slog.Info("get restaurant sponsorship", "request", req)

	response, err := s.sponsorService.GetRestaurantSponsorship(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &pb.SponsorshipResponse{
		Id:    response.ID,
		Tier:  int32(response.Tier),
		Until: timestamppb.New(response.Until),
	}, nil
}

func (s *server) Sponsor(ctx context.Context, req *pb.SponsorshipRequest) (*pb.SponsorshipResponse, error) {
	if req.Tier < 1 || req.Tier > 4 {
		return nil, status.Error(codes.InvalidArgument, "Tier must be between 1 and 4")
	}

	user, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	slog.Info("USER", "username", user.Username, "userID", user.UserID)

	sponsorship := &models.Sponsorship{
		ID:         req.Id,
		Tier:       int(req.Tier),
		Until:      time.Now().AddDate(0, 1, 0),
		Categories: []string{},
	}

	response, receipt_id, err := s.sponsorService.Sponsor(ctx, sponsorship, user.UserID, user.Email)
	if err != nil {
		return nil, err
	}

	slog.Info("ADDED TO DATABASE",
		"id", response.ID,
		"tier", response.Tier,
		"until", response.Until,
	)

	return &pb.SponsorshipResponse{
		Id:        response.ID,
		Tier:      int32(response.Tier),
		Until:     timestamppb.New(response.Until),
		ReceiptId: receipt_id,
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
	userID := uuidToInt64(claims.UserID)

	userInfo := &UserInfo{
		UserID:   userID,
		Email:    claims.Email,
		Username: claims.Username,
	}

	slog.Info("USER INFO",
		"user_id_claim", claims.UserID,
		"user_id", userID,
		"username", claims.Username,
		"email", claims.Email,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return userInfo, nil
}

// uuidToInt64 hashes a UUID to a positive int64 via FNV-64a.
// Matches the implementation in the auth service.
func uuidToInt64(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() &^ (uint64(1) << 63))
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

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "sponsor")

	slog.Info("server running", "addr", addr)
	if err := s.Serve(lis); err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
}

func NewRestaurantClient(address string) (pb.RestaurantToSponsorServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewRestaurantToSponsorServiceClient(conn), conn, nil
}

func NewPaymentClient(address string) (pb.PaymentCommandServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewPaymentCommandServiceClient(conn), conn, nil
}
