package app

import (
	"MrFood/services/payment/config"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"

	pb "MrFood/services/payment/internal/api/grpc/pb"
	models "MrFood/services/payment/pkg"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type paymentService interface {
	CreateReceipt(ctx context.Context, payment_request *models.PaymentRequest) (int32, error)
	GetReceiptByID(ctx context.Context, receipt_id int32) (*models.Receipt, error)
	GetReceiptsByUser(ctx context.Context, user_id int32) ([]*models.Receipt, error)
}

type Server struct {
	pb.UnimplementedPaymentServiceServer
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterPaymentServiceServer(s, &Server{})

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
