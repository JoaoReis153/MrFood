package app

import (
	pb "MrFood/services/sponsor/internal/api/grpc/pb"
	"MrFood/services/sponsor/internal/service"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedSponsorServiceServer
	sponsorService *service.Service
}

type UserInfo struct {
	UserID   int32
	Username string
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

func (s *server) PingPong(ctx context.Context, req *pb.Ping) (*pb.Pong, error) {
	return &pb.Pong{
		Id: 1,
	}, nil
}

func (s *server) Sponsor(ctx context.Context, req *pb.SponsorshipRequest) (*pb.SponsorshipResponse, error) {

	user, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &pb.SponsorshipResponse{
		Id:    user.UserID,
		Tier:  req.Tier,
		Until: timestamppb.New(time.Now().AddDate(0, 1, 0)),
	}, nil
}

func ExtractUserFromContext(ctx context.Context) (*UserInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return nil, errors.New("no auth header")
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")

	claims := &Claims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims)

	if err != nil {
		slog.Error("failed to parse token", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	userID, err := parseInt32(claims.UserID)

	if err != nil {
		slog.Error("failed to parse user id", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid user id in token")
	}

	userInfo := &UserInfo{
		UserID:   userID,
		Username: claims.Username,
	}

	slog.Info("USER INFO",
		"user_id", claims.UserID,
		"username", claims.Username,
		"token_type", claims.TokenType,
		"exp", claims.ExpiresAt,
	)

	return userInfo, nil
}

func parseInt32(value string) (int32, error) {
	v, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, errors.New("out of int32 range")
	}
	return int32(v), nil
}

func RunServer() {
	lis, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterSponsorServiceServer(s, &server{})

	fmt.Println("Server running on :50054")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
