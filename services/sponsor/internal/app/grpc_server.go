package app

import (
	pb "MrFood/services/sponsor/internal/api/grpc/pb"
	"MrFood/services/sponsor/internal/service"
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedSponsorServiceServer
	sponsorService *service.Service
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *server) ManyPings(stream pb.SponsorService_ManyPingsServer) error {
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

func (s *server) ManyPongs(req *pb.Ping, stream pb.SponsorService_ManyPongsServer) error {
	for i := 0; i < 5; i++ {
		err := stream.Send(&pb.Pong{Id: req.Id + int32(i)})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) ManyPingPongs(stream pb.SponsorService_ManyPingPongsServer) error {
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

func (s *server) Sponsor(ctx context.Context, req *pb.Sponsorship) (*pb.SponsorshipResponse, error) {
	return &pb.SponsorshipResponse{
		Id:   req.Id,
		Tier: req.Tier,
	}, nil
}

func RunServer() {
	lis, err := net.Listen("tcp", ":50055")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterSponsorServiceServer(s, &server{})

	fmt.Println("Server running on :50055")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
