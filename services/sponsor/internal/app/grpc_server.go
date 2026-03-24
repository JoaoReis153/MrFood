package app

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "MrFood/services/sponsor/internal/api/grpc/pb"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedTemplateServiceServer
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *server) ManyPings(stream pb.TemplateService_ManyPingsServer) error {
	var lastID int32

	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}
		lastID = req.Id
	}

	return stream.SendAndClose(&pb.Pong{Id: lastID})
}

func (s *server) ManyPongs(req *pb.Ping, stream pb.TemplateService_ManyPongsServer) error {
	for i := 0; i < 5; i++ {
		err := stream.Send(&pb.Pong{Id: req.Id + int32(i)})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) ManyPingPongs(stream pb.TemplateService_ManyPingPongsServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}
		err = stream.Send(&pb.Pong{Id: req.Id})
		if err != nil {
			return err
		}
	}
	return nil
}

func RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterTemplateServiceServer(s, &server{})

	fmt.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
