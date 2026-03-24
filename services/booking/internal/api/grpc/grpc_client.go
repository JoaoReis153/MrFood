package grpc

import (
	pb "MrFood/services/booking/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewClient() (pb.RestaurantServiceClient, func(), error) {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { conn.Close() }

	return pb.NewRestaurantServiceClient(conn), cleanup, nil
}
