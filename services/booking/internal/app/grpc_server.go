package app

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"

	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/service"
	models "MrFood/services/booking/pkg"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedBookingServiceServer
	bookingService *service.Service
}

func RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterBookingServiceServer(s, &server{})

	fmt.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func (s *server) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.Booking, error) {
	// calls client to get available slots
	res, err := s.bookingService.Client.GetSlots(ctx,
		&pb.GetSlotsRequest{
			RestaurantId: req.RestaurantId,
			TimeStart:    req.BookingRange.TimeStart,
		})

	if err != nil {
		slog.Error("Failed to get slots", "error", err)
		return nil, status.Error(codes.Internal, "failed to get slots")
	}

	booking := &models.Booking{
		UserID:       req.UserId,
		RestaurantID: req.RestaurantId,
		TimeStart:    req.BookingRange.TimeStart.AsTime(),
		TimeEnd:      req.BookingRange.TimeEnd.AsTime(),
		PeopleCount:  req.Quantity,
	}

	slots := &models.HourSlots{
		MaxSlots:     res.MaxSlots,
		CurrentSlots: res.CurrentSlots,
	}

	newBooking, err := s.bookingService.CreateBooking(ctx, booking, slots)

	if err != nil {
		slog.Error("Failed to get slots", "error", err)
		return nil, status.Error(codes.Internal, "failed to get slots")
	}

	slog.Info("Booking created", "user_id", req.UserId, "restaurant_id", req.RestaurantId, "time_start", req.BookingRange.TimeStart)

	return &pb.Booking{
		Id:           newBooking.ID,
		UserId:       newBooking.UserID,
		RestaurantId: newBooking.RestaurantID,
		Quantity:     newBooking.PeopleCount,
		BookingRange: &pb.TimeRange{
			TimeStart: timestamppb.New(newBooking.TimeStart),
			TimeEnd:   timestamppb.New(newBooking.TimeEnd),
		},
	}, nil
}
