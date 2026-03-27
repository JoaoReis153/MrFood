package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	pb "MrFood/services/restaurant/internal/api/grpc/pb"
	"MrFood/services/restaurant/internal/service"
	models "MrFood/services/restaurant/pkg"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedRestaurantServiceServer
	pb.UnimplementedRestaurantToBookingServiceServer
	pb.UnimplementedRestaurantToReviewServiceServer
	restaurantService restaurantService
}

type restaurantService interface {
	GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error)
	CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error)
	UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int32) (*models.Restaurant, error)
	CompareRestaurants(ctx context.Context, id1, id2 int32) (*models.Restaurant, *models.Restaurant, error)
	GetWorkingHours(ctx context.Context, restaurantID int32, timeStart time.Time) (*models.TimeRange, error)
	GetRestaurantStats(ctx context.Context, restaurantID int32) (*models.RestaurantStats, error)
}

func (s *server) GetRestaurantDetails(ctx context.Context, req *pb.GetRestaurantDetailsRequest) (*pb.GetRestaurantDetailsResponse, error) {
	slog.Info("fetching restaurant details", "restaurant_id", req.GetId())

	restaurant, err := s.restaurantService.GetRestaurantByID(ctx, req.GetId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.GetRestaurantDetailsResponse{
		Restaurant: modelToPB(restaurant),
	}, nil
}

func (s *server) CreateRestaurant(ctx context.Context, req *pb.CreateRestaurantRequest) (*pb.CreateRestaurantResponse, error) {
	requesterOwnerID, err := ownerIDFromMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	slog.Info("creating restaurant", "name", req.GetName(), "owner_id", requesterOwnerID)

	restaurant := &models.Restaurant{
		OwnerID:      requesterOwnerID,
		Name:         req.GetName(),
		Address:      req.GetAddress(),
		WorkingHours: req.GetWorkingHours(),
		Categories:   req.GetCategories(),
		Latitude:     req.GetLatitude(),
		Longitude:    req.GetLongitude(),
		MaxSlots:     req.GetMaxSlots(),
	}

	newRestaurantID, err := s.restaurantService.CreateRestaurant(ctx, restaurant)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.CreateRestaurantResponse{
		RestaurantId: newRestaurantID,
		PresignedUrl: nil, // get pre-signed url later
	}, nil
}

func (s *server) UpdateRestaurant(ctx context.Context, req *pb.UpdateRestaurantRequest) (*pb.UpdateRestaurantResponse, error) {
	requesterOwnerID, err := ownerIDFromMetadata(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	changes := &models.Restaurant{
		ID:         req.GetId(),
		Name:       req.GetName(),
		Address:    req.GetAddress(),
		Categories: req.GetCategories(),
		Latitude:   req.GetLatitude(),
		Longitude:  req.GetLongitude(),
		MaxSlots:   req.GetMaxSlots(),
	}
	for _, wh := range req.GetWorkingHours() {
		if wh == nil {
			continue
		}
		changes.WorkingHours = append(changes.WorkingHours, wh.AsTime().UTC().Format(time.RFC3339))
	}

	updatedRestaurant, err := s.restaurantService.UpdateRestaurant(ctx, changes, requesterOwnerID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.UpdateRestaurantResponse{Restaurant: modelToPB(updatedRestaurant)}, nil
}

func (s *server) CompareRestaurantDetails(ctx context.Context, req *pb.CompareRestaurantDetailsRequest) (*pb.CompareRestaurantDetailsResponse, error) {
	r1, r2, err := s.restaurantService.CompareRestaurants(ctx, req.GetRestaurantId_1(), req.GetRestaurantId_2())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.CompareRestaurantDetailsResponse{
		Restaurant1: modelToPB(r1),
		Restaurant2: modelToPB(r2),
	}, nil
}

func (s *server) GetWorkingHours(ctx context.Context, req *pb.WorkingHoursRequest) (*pb.WorkingHoursResponse, error) {
	var requestedAt time.Time
	if req.GetTimeStart() != nil {
		requestedAt = req.GetTimeStart().AsTime().UTC()
	}

	workingHours, err := s.restaurantService.GetWorkingHours(ctx, req.GetRestaurantId(), requestedAt)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.WorkingHoursResponse{
		RestaurantId: req.GetRestaurantId(),
		WorkingHours: &pb.TimeRange{
			TimeStart: timestamppb.New(workingHours.TimeStart),
			TimeEnd:   timestamppb.New(workingHours.TimeEnd),
		},
	}, nil
}

func (s *server) GetRestaurantStats(ctx context.Context, req *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error) {
	stats, err := s.restaurantService.GetRestaurantStats(ctx, req.GetRestaurantId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.GetRestaurantStatsResponse{
		RestaurantStats: &pb.RestaurantStats{
			RestaurantId:  stats.RestaurantID,
			AverageRating: stats.AverageRating,
			ReviewCount:   stats.ReviewCount,
		},
	}, nil
}

func (app *App) RunServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	srv := &server{
		restaurantService: app.Service,
	}

	pb.RegisterRestaurantServiceServer(s, srv)
	pb.RegisterRestaurantToBookingServiceServer(s, srv)
	pb.RegisterRestaurantToReviewServiceServer(s, srv)

	slog.Info("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
}

func ownerIDFromMetadata(ctx context.Context) (int32, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, errors.New("missing request metadata")
	}

	ownerIDs := md.Get("x-user-id")
	if len(ownerIDs) == 0 {
		return 0, errors.New("missing x-user-id maetadata")
	}

	ownerID := strings.TrimSpace(ownerIDs[0])
	if ownerID == "" {
		return 0, errors.New("empty x-user-id metadata")
	}

	parsed, err := parseInt32(ownerID)
	if err != nil {
		return 0, errors.New("invalid x-user-id metadata")
	}

	return parsed, nil
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

func modelToPB(restaurant *models.Restaurant) *pb.RestaurantDetails {
	if restaurant == nil {
		return nil
	}

	response := &pb.RestaurantDetails{
		Id:          restaurant.ID,
		Name:        restaurant.Name,
		Latitude:    restaurant.Latitude,
		Longitude:   restaurant.Longitude,
		Address:     restaurant.Address,
		Categories:  restaurant.Categories,
		MaxSlots:    restaurant.MaxSlots,
		OwnerId:     restaurant.OwnerID,
		OwnerName:   restaurant.OwnerName,
		SponsorTier: restaurant.SponsorTier,
	}

	if strings.TrimSpace(restaurant.MediaURL) != "" {
		mediaURL := restaurant.MediaURL
		response.MediaUrl = &mediaURL
	}

	for _, wh := range restaurant.WorkingHours {
		if ts := parseTimestampToProto(wh); ts != nil {
			response.WorkingHours = append(response.WorkingHours, ts)
		}
	}

	return response
}

func parseTimestampToProto(value string) *timestamppb.Timestamp {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return timestamppb.New(parsed.UTC())
		}
	}
	return nil
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidRestaurant), errors.Is(err, service.ErrInvalidCompareRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrRestaurantAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		slog.Error("restaurant rpc failed", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}
