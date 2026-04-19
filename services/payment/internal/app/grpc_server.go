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
	"strconv"
	"strings"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
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
		s.GracefulStop()
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
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user_id, err := parseInt64(claims.UserID)
	if err != nil {
		return nil, err
	}

	request := &models.Receipt{
		IdempotencyKey:     req.IdempotencyKey,
		Amount:             req.Amount,
		PaymentDescription: req.PaymentDescription,
		PaymentType:        req.Type,
		UserEmail:          claims.UserEmail,
		UserID:             user_id,
	}

	receipt_id, err := s.paymentService.CreateReceipt(ctx, request)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.PaymentResponse{
		ReceiptId: receipt_id,
	}, nil
}

func (s *queryServer) GetReceipt(ctx context.Context, req *pb.ReceiptRequest) (*pb.GetReceiptResponse, error) {
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user_id, err := parseInt64(claims.UserID)
	if err != nil {
		return nil, err
	}

	err = s.paymentService.GetReceiptById(ctx, *req.ReceiptId, user_id)

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
	case errors.Is(err, service.ErrReceiptsNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		slog.Error("restaurant rpc failed", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	UserEmail    string `json:"user_email"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

func parseInt64(value string) (int64, error) {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, errors.New("out of int64 range")
	}
	return int64(v), nil
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
		"user_email", claims.UserEmail,
		"username", claims.Username,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return claims, nil
}
