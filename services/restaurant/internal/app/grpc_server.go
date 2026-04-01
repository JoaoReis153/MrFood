package app

import (
	"MrFood/services/restaurant/config"
	pb "MrFood/services/restaurant/internal/api/grpc/pb"
	"MrFood/services/restaurant/internal/service"
	models "MrFood/services/restaurant/pkg"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedRestaurantServiceServer
	pb.UnimplementedRestaurantToBookingServiceServer
	pb.UnimplementedReviewToRestaurantServiceServer
	pb.UnimplementedRestaurantToSponsorServiceServer
	restaurantService restaurantService
}

type restaurantService interface {
	GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error)
	GetRestaurantID(ctx context.Context, id int32) (int32, error)
	CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error)
	UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int32) (*models.Restaurant, error)
	CompareRestaurants(ctx context.Context, id1, id2 int32) (*models.Restaurant, *models.Restaurant, error)
	GetWorkingHours(ctx context.Context, restaurantID int32, timeStart time.Time) (*models.TimeRange, error)
}

type reviewStatsClient struct {
	client pb.RestaurantToReviewServiceClient
}

func newReviewStatsClient(target string) (*reviewStatsClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial review grpc: %w", err)
	}

	return &reviewStatsClient{client: pb.NewRestaurantToReviewServiceClient(conn)}, conn, nil
}

func (c *reviewStatsClient) GetRestaurantStats(ctx context.Context, restaurantID int32) (*models.RestaurantStats, error) {
	reviewCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	resp, err := c.client.GetRestaurantStats(reviewCtx, &pb.GetRestaurantStatsRequest{RestaurantId: restaurantID})
	if err != nil {
		code := status.Code(err)
		if code == codes.DeadlineExceeded || code == codes.Unavailable {
			return nil, nil
		}
		return nil, nil
	}

	stats := resp.GetRestaurantStats()
	if stats == nil {
		return &models.RestaurantStats{RestaurantID: restaurantID}, nil
	}

	return &models.RestaurantStats{
		RestaurantID:  stats.GetRestaurantId(),
		AverageRating: stats.GetAverageRating(),
		ReviewCount:   stats.GetReviewCount(),
	}, nil
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

func (s *server) GetRestaurantId(ctx context.Context, req *pb.GetRestaurantRequest) (*pb.GetRestaurantResponse, error) {
	slog.Info("fetching restaurant id for review service", "restaurant_id", req.GetRestaurantId())
	restaurantID, err := s.restaurantService.GetRestaurantID(ctx, req.GetRestaurantId())
	if err != nil {
		slog.Error("failed to get restaurant id", "error", err)
		return nil, mapServiceError(err)
	}
	return &pb.GetRestaurantResponse{
		RestaurantId: restaurantID,
	}, nil
}

func (s *server) CreateRestaurant(ctx context.Context, req *pb.CreateRestaurantRequest) (*pb.CreateRestaurantResponse, error) {
	requesterOwner, err := ExtractUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	slog.Info("creating restaurant", "name", req.GetName(), "owner_id", requesterOwner.UserID)

	restaurant := &models.Restaurant{
		OwnerID:      requesterOwner.UserID,
		OwnerName:    requesterOwner.Username,
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
	requestOwner, err := ExtractUserFromContext(ctx)

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

	updatedRestaurant, err := s.restaurantService.UpdateRestaurant(ctx, changes, requestOwner.UserID)
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
		TimeStart:    timestamppb.New(workingHours.TimeStart),
		TimeEnd:      timestamppb.New(workingHours.TimeEnd),
	}, nil
}

func (s *server) GetRestaurantSponsorship(ctx context.Context, req *pb.GetRestaurantSponsorshipRequest) (*pb.GetRestaurantSponsorshipResponse, error) {
	slog.Info("fetching restaurant details", "restaurant_id", req.GetId())

	restaurant, err := s.restaurantService.GetRestaurantByID(ctx, req.GetId())
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &pb.GetRestaurantSponsorshipResponse{
		Id:         restaurant.ID,
		Categories: restaurant.Categories,
		OwnerId:    restaurant.OwnerID,
	}, nil
}

func (app *App) RunServer() {
	cfg := config.Get(context.Background())
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	lis, err := net.Listen("tcp", addr)
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
	pb.RegisterReviewToRestaurantServiceServer(s, srv)
	pb.RegisterRestaurantToSponsorServiceServer(s, srv)

	slog.Info("server running", "addr", addr)
	if err := s.Serve(lis); err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

type UserInfo struct {
	UserID   int32
	Username string
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

	if restaurant.AverageRating != nil {
		averageRating := *restaurant.AverageRating
		response.AverageRating = &averageRating
	}
	if restaurant.ReviewCount != nil {
		reviewCount := *restaurant.ReviewCount
		response.ReviewCount = &reviewCount
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
