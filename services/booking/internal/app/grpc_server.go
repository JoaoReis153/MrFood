package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"

	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/service"
	models "MrFood/services/booking/pkg"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type bookingService interface {
	CreateBooking(ctx context.Context, booking *models.Booking) (int32, error)
	DeleteBooking(ctx context.Context, booking *models.Booking) error
}

type server struct {
	pb.UnimplementedBookingServiceServer
	bookingService bookingService
}

func RunServer(service bookingService) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterBookingServiceServer(s, &server{
		bookingService: service,
	})
	reflection.Register(s)

	fmt.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func NewClient() (pb.RestaurantToBookingServiceClient, func(), error) {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { conn.Close() }

	return pb.NewRestaurantToBookingServiceClient(conn), cleanup, nil
}

func (s *server) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.CreateBookingResponse, error) {
	slog.Info("received booking CREATION request", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart, "people_count", req.Quantity)

	booking := &models.Booking{
		UserID:       req.UserId,
		RestaurantID: req.RestaurantId,
		TimeStart:    req.TimeStart.AsTime(),
		PeopleCount:  req.Quantity,
	}

	booking_id, err := s.bookingService.CreateBooking(ctx, booking)

	if err != nil {
		return nil, mapServiceError(err)
	}

	slog.Info("Booking created", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart)

	return &pb.CreateBookingResponse{
		BookingId: booking_id,
	}, nil
}

func (s *server) DeleteBooking(ctx context.Context, req *pb.DeleteBookingRequest) (*pb.DeleteBookingResponse, error) {
	slog.Info("received booking DELETION request", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart)

	booking := &models.Booking{
		UserID:       req.UserId,
		RestaurantID: req.RestaurantId,
		TimeStart:    req.TimeStart.AsTime(),
	}

	err := s.bookingService.DeleteBooking(ctx, booking)

	if err != nil {
		return nil, mapServiceError(err)
	}

	slog.Info("Booking deleted", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart)

	return &pb.DeleteBookingResponse{}, nil
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
