package grpc

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"strings"

	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/service"
	models "MrFood/services/booking/pkg"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedBookingServiceServer
	bookingService *service.Service
}

func RunServer(service *service.Service) {
	lis, err := net.Listen("tcp", ":50060")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterBookingServiceServer(s, &server{
		bookingService: service,
	})
	reflection.Register(s)

	fmt.Println("Server running on :50060")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func NewClient() (pb.RestaurantServiceClient, func(), error) {
	conn, err := grpc.NewClient(
		"localhost:50060",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { conn.Close() }

	return pb.NewRestaurantServiceClient(conn), cleanup, nil
}

func (s *server) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.Booking, error) {
	slog.Info("received booking request", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart, "people_count", req.Quantity)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		slog.Info("no metadata")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		slog.Info("no auth header")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	slog.Info("TOKEN: " + token)

	res, err := s.bookingService.Client.GetWorkingHours(ctx,
		&pb.WorkingHoursRequest{
			RestaurantId: req.RestaurantId,
			TimeStart:    req.TimeStart,
		})

	if err != nil {
		slog.Error("Failed to get slots", "error", err)
		return nil, status.Error(codes.Internal, "failed to get slots")
	}

	booking := &models.Booking{
		UserID:       req.UserId,
		RestaurantID: req.RestaurantId,
		TimeStart:    req.TimeStart.AsTime(),
		PeopleCount:  req.Quantity,
	}

	// Mock response from gRPC
	// res := &pb.WorkingHoursResponse{
	// 	RestaurantId: 1,
	// 	WorkingHours: &pb.TimeRange{
	// 		TimeStart: timestamppb.New(time.Date(2026, 3, 24, 9, 0, 0, 0, time.UTC)),  // 9:00 AM
	// 		TimeEnd:   timestamppb.New(time.Date(2026, 3, 24, 18, 0, 0, 0, time.UTC)), // 6:00 PM
	// 	},
	// }

	working_hours := &models.WorkingHours{
		RestaurantID: res.RestaurantId,
		TimeStart:    res.WorkingHours.TimeStart.AsTime(),
		TimeEnd:      res.WorkingHours.TimeEnd.AsTime(),
	}

	newBooking, err := s.bookingService.CreateBooking(ctx, booking, working_hours)

	if err != nil {
		slog.Error("Internal service error", "error", err)
		return nil, status.Error(codes.Internal, "internal service error")
	}

	slog.Info("Booking created", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.TimeStart)

	return &pb.Booking{
		Id:           newBooking.ID,
		UserId:       newBooking.UserID,
		RestaurantId: newBooking.RestaurantID,
		Quantity:     newBooking.PeopleCount,
		WorkingHours: &pb.TimeRange{
			TimeStart: timestamppb.New(newBooking.TimeStart),
			TimeEnd:   timestamppb.New(newBooking.TimeEnd),
		},
	}, nil
}
