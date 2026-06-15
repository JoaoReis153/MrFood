package app

import (
	"MrFood/services/payment/config"
	"MrFood/services/payment/internal/api/webhook"
	"MrFood/services/payment/internal/service"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stripe/stripe-go/v85"
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
	CreateReceipt(ctx context.Context, receipt *models.Receipt) (int32, error)
	ConfirmPayment(ctx context.Context, paymentIntentID string) error
}

type paymentQueryService interface {
	GetReceiptById(ctx context.Context, receiptID int32, userID int64) error
	GetReceiptsByUser(ctx context.Context, userID int64) error
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
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.runGRPC(ctx, cfg)
	})

	g.Go(func() error {
		return app.runWebhook(ctx, cfg)
	})

	return g.Wait()
}

func (app *App) runGRPC(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed to listen", "port", cfg.Server.Port, "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterPaymentCommandServiceServer(s, &commandServer{paymentService: app.Service})
	pb.RegisterPaymentQueryServiceServer(s, &queryServer{paymentService: app.Service})

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("gRPC server listening", "port", cfg.Server.Port)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
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

func (app *App) runWebhook(ctx context.Context, cfg *config.Config) error {
	webhookHandler := webhook.New(cfg.Stripe.WebhookSecret, app.Service.ConfirmPayment)

	mux := http.NewServeMux()
	mux.Handle("POST /webhook", webhookHandler)

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Webhook.Port),
		Handler: mux,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("webhook server listening", "port", cfg.Webhook.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("webhook serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		slog.Info("shutting down webhook server")
		return srv.Shutdown(context.Background())
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

// gRPC handlers
func (s *commandServer) MakePayment(ctx context.Context, req *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	slog.InfoContext(ctx, "creating payment", "user_id", req.UserId, "amount", req.Amount, "type", req.Type)

	receipt := &models.Receipt{
		IdempotencyKey:     req.IdempotencyKey,
		Amount:             req.Amount,
		PaymentDescription: req.PaymentDescription,
		PaymentType:        req.Type,
		UserEmail:          req.UserEmail,
		UserID:             req.UserId,
	}

	receiptID, err := s.paymentService.CreateReceipt(ctx, receipt)
	if err != nil {
		if errors.Is(err, service.ErrDuplicatePaymentRequest) {
			slog.WarnContext(ctx, "duplicate payment request", "idempotency_key", req.IdempotencyKey)
		} else {
			slog.ErrorContext(ctx, "failed to create receipt", "error", err)
		}
		return nil, mapServiceError(err)
	}

	return &pb.PaymentResponse{ReceiptId: receiptID}, nil
}

func (s *queryServer) GetReceiptsByUser(ctx context.Context, _ *pb.ReceiptRequest) (*pb.GetReceiptResponse, error) {

	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.paymentService.GetReceiptsByUser(ctx, uuidToInt64(claims.UserID)); err != nil {
		if errors.Is(err, service.ErrReceiptNotFound) {
			slog.InfoContext(ctx, "no receipts found for user", "user_id", claims.UserID)
		} else {
			slog.ErrorContext(ctx, "failed to get receipts by user", "user_id", claims.UserID, "error", err)
		}
		return nil, mapServiceError(err)
	}

	return &pb.GetReceiptResponse{}, nil
}

func (s *queryServer) GetReceiptById(ctx context.Context, req *pb.ReceiptRequest) (*pb.GetReceiptResponse, error) {

	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.paymentService.GetReceiptById(ctx, req.ReceiptId, uuidToInt64(claims.UserID)); err != nil {
		if errors.Is(err, service.ErrReceiptNotFound) {
			slog.InfoContext(ctx, "receipt not found", "receipt_id", req.ReceiptId)
		} else {
			slog.ErrorContext(ctx, "failed to get receipt", "receipt_id", req.ReceiptId, "error", err)
		}
		return nil, mapServiceError(err)
	}

	return &pb.GetReceiptResponse{}, nil
}

func mapServiceError(err error) error {
	// Context errors
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request timed out")
	}
	// Stripe errors
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) {
		if stripeErr.HTTPStatusCode >= 400 && stripeErr.HTTPStatusCode < 500 {
			return status.Error(codes.InvalidArgument, stripeErr.Msg)
		}
		return status.Error(codes.Unavailable, "payment provider unavailable")
	}
	// Service errors
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
	TokenType    string `json:"token_type"`
}

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

	if _, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims); err != nil {
		slog.WarnContext(ctx, "invalid token", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return claims, nil
}
