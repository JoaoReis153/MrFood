package app

import (
	"MrFood/services/payment/config"
	"MrFood/services/payment/internal/service"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"hash/fnv"
	"strconv"
	"strings"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type paymentCommandService interface {
	CreateReceipt(ctx context.Context, payment_request *models.Receipt) (int32, error)
}

type paymentQueryService interface {
	GetReceiptById(ctx context.Context, receipt_id int32, user_id int64) error
	GetReceiptsByUser(ctx context.Context, user_id int64) error
}

type commandServer struct {
	pb.UnimplementedPaymentCommandServiceServer
	paymentService paymentCommandService
}

type queryServer struct {
	pb.UnimplementedPaymentQueryServiceServer
	paymentService paymentQueryService
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterPaymentCommandServiceServer(s, &commandServer{
		paymentService: app.Service,
	})
	pb.RegisterPaymentQueryServiceServer(s, &queryServer{
		paymentService: app.Service,
	})
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "payment")

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
		slog.Info("shutting down gRPC server...")
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}

func NewClient(address string) (pb.PaymentToNotificationServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewPaymentToNotificationServiceClient(conn), conn, nil
}

func (s *commandServer) MakePayment(ctx context.Context, req *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	slog.Info("received request for payment")

	request := &models.Receipt{
		IdempotencyKey:     req.IdempotencyKey,
		Amount:             req.Amount,
		PaymentDescription: req.PaymentDescription,
		PaymentType:        req.Type,
		UserEmail:          req.UserEmail,
		UserID:             req.UserId,
	}

	receipt_id, err := s.paymentService.CreateReceipt(ctx, request)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.PaymentResponse{
		ReceiptId: receipt_id,
	}, nil
}

func (s *queryServer) GetReceiptsByUser(ctx context.Context, req *pb.ReceiptRequest) (*pb.GetReceiptResponse, error) {
	slog.Info("received request for receipts by user")

	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user_id := uuidToInt64(claims.UserID)

	err = s.paymentService.GetReceiptsByUser(ctx, user_id)

	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.GetReceiptResponse{}, nil
}

func (s *queryServer) GetReceiptById(ctx context.Context, req *pb.ReceiptRequest) (*pb.GetReceiptResponse, error) {
	slog.Info("received request for receipt by id")

	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user_id := uuidToInt64(claims.UserID)

	err = s.paymentService.GetReceiptById(ctx, req.ReceiptId, user_id)

	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.GetReceiptResponse{}, nil
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidAmmount), errors.Is(err, service.ErrNullIdempotencyKey):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrDuplicatePaymentRequest):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrReceiptNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		slog.Error("payment rpc failed", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

// uuidToInt64 hashes a UUID to a positive int64 via FNV-64a.
// Matches the implementation in the auth service.
func uuidToInt64(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() &^ (uint64(1) << 63))
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
		"email", claims.Email,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return claims, nil
}
