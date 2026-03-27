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

	"github.com/golang-jwt/jwt/v5"
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

	id, err := GetUserIDFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get id", "error", err)
		return nil, status.Error(codes.Internal, "failed to get id")
	}

	slog.Info("USERID FROM AUTH: " + id)
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

// extremely wrong way of doing this, very temporary, this should be called from auth, or every service should have the jwt properties in their config but that seems wrong
func GetUserIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return "", fmt.Errorf("no auth header")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	slog.Info("TOKEN: " + token)

	return ValidateToken(token)
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func ValidateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("this-is-a-temporary-secret-that-we-should-probably-change"), nil
	})

	slog.Info("VALIDATION ATTEMPT: " + token.Claims.(*Claims).UserID)

	if err != nil || !token.Valid {
		return "", err
	}

	claims := token.Claims.(*Claims)
	return claims.UserID, nil
}
