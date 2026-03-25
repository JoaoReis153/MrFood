package app

import (
	"context"
	"log/slog"
	"time"

	pb "MrFood/services/booking/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewClient() (pb.RestaurantServiceClient, func(), error) {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		conn.Close()
	}

	return pb.NewRestaurantServiceClient(conn), cleanup, nil
}

// called by the server whenever a booking request is received - gets available slots if time_start is within a working hour
func GetWorkingHours(client pb.RestaurantServiceClient, ctx context.Context, restaurant_id int32, time_start time.Time) (*pb.WorkingHoursResponse, error) {
	res, err := client.GetWorkingHours(ctx,
		&pb.WorkingHoursRequest{
			RestaurantId: restaurant_id,
			TimeStart:    timestamppb.New(time_start),
		})
	if err != nil {
		slog.Error("Unable to get working hours", "error", err)
		return nil, status.Error(codes.Internal, "failed to get working hours")
	}

	return res, nil
}
