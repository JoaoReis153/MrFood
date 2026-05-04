package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"hash/fnv"
	"strings"

	"MrFood/services/booking/config"
	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/service"
	models "MrFood/services/booking/pkg"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type bookingService interface {
	CreateBooking(ctx context.Context, booking *models.Booking) (int32, int32, error)
	DeleteBooking(ctx context.Context, delete_request *models.DeleteBooking) error
}

type server struct {
	pb.UnimplementedBookingServiceServer
	bookingService bookingService
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

func RunServer(service bookingService) {
	cfg := config.Get(context.Background())
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterBookingServiceServer(s, &server{
		bookingService: service,
	})
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "booking")

	fmt.Println("Server running on", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func NewRestaurantClient(address string) (pb.RestaurantToBookingServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewRestaurantToBookingServiceClient(conn), conn, nil
}

func NewPaymentClient(address string) (pb.PaymentCommandServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewPaymentCommandServiceClient(conn), conn, nil
}

func (s *server) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.CreateBookingResponse, error) {

	claims, err := ExtractUserFromContext(ctx)

	if err != nil {
		return nil, err
	}

	user_id := uuidToInt64(claims.Subject)

	slog.Info("received booking CREATION request", "user_id", user_id, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart, "people_count", req.Quantity)

	booking := &models.Booking{
		UserID:       user_id,
		UserEmail:    claims.Email,
		RestaurantID: req.RestaurantId,
		TimeStart:    req.TimeStart.AsTime(),
		PeopleCount:  req.Quantity,
	}

	booking_id, receipt_id, err := s.bookingService.CreateBooking(ctx, booking)

	if err != nil {
		return nil, mapServiceError(err)
	}

	slog.Info("Booking created", "user_id", user_id, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart)

	return &pb.CreateBookingResponse{
		BookingId: booking_id,
		Payment: &pb.PaymentResponse{
			ReceiptId: receipt_id,
		},
	}, nil
}

func (s *server) DeleteBooking(ctx context.Context, req *pb.DeleteBookingRequest) (*pb.DeleteBookingResponse, error) {
	claims, err := ExtractUserFromContext(ctx)

	if err != nil {
		return nil, err
	}

	user_id := uuidToInt64(claims.Subject)

	delete_request := &models.DeleteBooking{
		BookingID: req.BookingId,
		UserID:    user_id,
	}

	slog.Info("received booking DELETION request", "booking_id", delete_request.BookingID)

	err = s.bookingService.DeleteBooking(ctx, delete_request)

	if err != nil {
		return nil, mapServiceError(err)
	}

	slog.Info("booking DELETED", "booking_id", delete_request.BookingID)

	return &pb.DeleteBookingResponse{}, nil
}

func ExtractUserFromContext(ctx context.Context) (*Claims, error) {
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

	slog.Info("USER INFO",
		"user_id", claims.UserID,
		"username", claims.Username,
		"user_email", claims.Email,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return claims, nil
}

// uuidToInt64 hashes a UUID to a positive int64 via FNV-64a.
// Matches the implementation in the auth service.
func uuidToInt64(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() &^ (uint64(1) << 63))
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidBooking):
		slog.Error("Invalid booking fields", "error", err)
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrBookingAlreadyExists):
		slog.Error("Booking already exists", "error", err)
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrForbidden):
		slog.Error("Permission denied", "error", err)
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrBookingNotFound):
		slog.Error("Booking not found", "error", err)
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrFailedWHGet):
		slog.Error("Failed to get working hours", "error", err)
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		slog.Error("internal service error", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}
