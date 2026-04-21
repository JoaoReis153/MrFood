package app

import (
	"MrFood/services/notification/config"
	"MrFood/services/notification/internal/service"
	models "MrFood/services/notification/pkg"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"

	pb "MrFood/services/notification/internal/api/grpc/pb"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedAuthToNotificationServiceServer
	pb.UnimplementedPaymentToNotificationServiceServer
	svc *service.Service
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterAuthToNotificationServiceServer(s, &Server{svc: app.NotificationService})
	pb.RegisterPaymentToNotificationServiceServer(s, &Server{svc: app.NotificationService})

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("notification", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "notification")

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
		healthServer.SetServingStatus("notification", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}

func (s *Server) SendRegistrationEmail(ctx context.Context, req *pb.SendRegistrationEmailRequest) (*pb.SendRegistrationEmailResponse, error) {
	slog.Info("SendRegistrationEmail", "email", req.GetEmail(), "username", req.GetUsername())

	_, err := s.svc.SendRegistrationEmail(ctx, req)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	return &pb.SendRegistrationEmailResponse{}, nil
}

func (s *Server) SendReceipts(ctx context.Context, req *pb.SendReceiptsRequest) (*pb.SendReceiptsResponse, error) {
	slog.Info("SendReceipts", "email", req.UserEmail, "receiptCount", len(req.GetReceipts()))

	_, err := s.svc.SendReceipts(ctx, req)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	return &pb.SendReceiptsResponse{}, nil
}

func mapToGRPCError(err error) error {
	slog.Error("gRPC Operation Failed", "error", err)
	switch {
	case errors.Is(err, models.ErrInvalidEmail), errors.Is(err, models.ErrInvalidUsername), errors.Is(err, models.ErrEmptyReceipts):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, models.ErrRateLimitExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, models.ErrSendEmailFailed):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "Internal server error")
	}
}
