package app

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "MrFood/services/restaurant/internal/api/grpc/pb"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedRestaurantServiceServer
}

func (s *server) GetRestaurantDetails(ctx context.Context, req *pb.GetRestaurantDetailsRequest) (*pb.GetRestaurantDetailsResponse, error) {
	log.Printf("Fetching details for restaurant ID: %d", req.GetId())

	return &pb.GetRestaurantDetailsResponse{
		Restaurant: &pb.RestaurantDetails{
			Id:   req.GetId(),
			Name: "The Tasty Gopher",
		},
	}, nil
}

func (s *server) CreateRestaurant(ctx context.Context, req *pb.CreateRestaurantRequest) (*pb.CreateRestaurantResponse, error) {
	log.Printf("Creating restaurant: %s", req.GetName())

	return &pb.CreateRestaurantResponse{
		RestaurantId: 101,
		PresignedUrl: nil,
	}, nil
}

func (s *server) UpdateRestaurant(ctx context.Context, req *pb.UpdateRestaurantRequest) (*pb.UpdateRestaurantResponse, error) {
	return &pb.UpdateRestaurantResponse{}, nil
}

func (s *server) CompareRestaurantDetails(ctx context.Context, req *pb.CompareRestaurantDetailsRequest) (*pb.CompareRestaurantDetailsResponse, error) {
	return &pb.CompareRestaurantDetailsResponse{
		Restaurant1: &pb.RestaurantDetails{Id: req.RestaurantId_1},
		Restaurant2: &pb.RestaurantDetails{Id: req.RestaurantId_2},
	}, nil
}

func RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	// Register the correct service
	pb.RegisterRestaurantServiceServer(s, &server{})

	fmt.Println("Restaurant gRPC Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
