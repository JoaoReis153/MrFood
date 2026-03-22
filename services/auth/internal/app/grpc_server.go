package app

import (
	"MrFood/services/auth/internal/service"
	models "MrFood/services/auth/pkg"
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	pb "MrFood/services/auth/internal/api/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedTemplateServiceServer
	authService *service.Service
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (app *App) RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterTemplateServiceServer(s, &server{authService: app.Service})

	fmt.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
