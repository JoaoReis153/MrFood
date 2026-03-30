package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"MrFood/services/review/config"
	pb "MrFood/services/review/internal/api/grpc/pb"
	models "MrFood/services/review/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedReviewServiceServer
	pb.UnimplementedReviewToRestaurantServiceServer
	svc ReviewService
}

type RestaurantClient struct {
	client pb.ReviewToRestaurantServiceClient
}

func NewRestaurantClient(target string) (*RestaurantClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial restaurant grpc: %w", err)
	}
	return &RestaurantClient{client: pb.NewReviewToRestaurantServiceClient(conn)}, conn, nil
}

type ReviewService interface {
	GetReviews(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error)
	CreateReview(ctx context.Context, review models.Review) (models.Review, error)
	UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error)
	DeleteReview(ctx context.Context, deleteReq models.DeleteReview) error
	GetRestaurantStats(ctx context.Context, restaurantID int32) (models.RestaurantStats, error)
}

func (c *RestaurantClient) GetRestaurant(ctx context.Context, restaurantID int32) (models.Restaurant, error) {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	resp, err := c.client.GetRestaurantId(ctx, &pb.GetRestaurantRequest{RestaurantId: restaurantID})
	if err != nil {
		code := status.Code(err)
		if code == codes.DeadlineExceeded || code == codes.Unavailable {
			slog.Error("Restaurant service unavailable", "restaurantID", restaurantID, "error", err)
			return models.Restaurant{}, models.ErrRestaurantServiceUnavailable
		}
		slog.Error("Failed to get restaurant", "restaurantID", restaurantID, "error", err)
		return models.Restaurant{}, err
	}

	return models.Restaurant{
		RestaurantID: resp.GetRestaurantId(),
	}, nil
}

type Claims struct {
	jwt.RegisteredClaims
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"` // access or refresh
}

func (s *server) GetReviews(ctx context.Context, req *pb.GetReviewsRequest) (*pb.GetReviewsResponse, error) {
	slog.Info("Received GetReviews request", "restaurantID", req.GetRestaurantId(), "page", req.GetPage(), "limit", req.GetLimit())
	page, limit := int(req.GetPage()), int(req.GetLimit())
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 10
	}
	results, err := s.svc.GetReviews(ctx, int(req.GetRestaurantId()), page, limit)
	if err != nil {
		return nil, mapToGRPCError(err)
	}
	pbReviews := make([]*pb.Review, len(results.Reviews))
	for i, r := range results.Reviews {
		pbReviews[i] = &pb.Review{
			ReviewId:     r.ReviewID,
			RestaurantId: r.RestaurantID,
			UserId:       r.UserID,
			Rating:       r.Rating,
			Comment:      r.Comment,
			CreatedAt:    timestamppb.New(r.CreatedAt),
		}
	}
	slog.Info("GetReviews request successful", "restaurantID", req.GetRestaurantId(), "page", page, "limit", limit, "totalReviews", results.Pagination.Total)
	return &pb.GetReviewsResponse{
		Reviews: pbReviews,
		Pagination: &pb.Pagination{
			Page:  int32(results.Pagination.Page),
			Limit: int32(results.Pagination.Limit),
			Total: int32(results.Pagination.Total),
			Pages: int32(results.Pagination.Pages),
		},
	}, nil
}

func (s *server) CreateReview(ctx context.Context, req *pb.CreateReviewRequest) (*pb.CreateReviewResponse, error) {
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		slog.Error("Failed to extract user from context", "error", err)
		return nil, mapToGRPCError(err)
	}

	user_id, err := parseInt32(claims.UserID)
	if err != nil {
		slog.Error("Invalid user ID in token", "userID", claims.UserID, "error", err)
		return nil, mapToGRPCError(err)
	}
	slog.Info("Received CreateReview request", "restaurantID", req.GetRestaurantId(), "userID", user_id, "rating", req.GetRating())
	review := models.Review{
		RestaurantID: req.GetRestaurantId(),
		UserID:       user_id,
		Rating:       req.GetRating(),
		Comment:      req.GetComment(),
	}

	reviewResponse, err := s.svc.CreateReview(ctx, review)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("CreateReview request successful", "reviewID", reviewResponse.ReviewID, "restaurantID", reviewResponse.RestaurantID, "userID", reviewResponse.UserID)
	return &pb.CreateReviewResponse{
		Review: &pb.Review{
			ReviewId:     reviewResponse.ReviewID,
			RestaurantId: reviewResponse.RestaurantID,
			UserId:       reviewResponse.UserID,
			Rating:       reviewResponse.Rating,
			Comment:      reviewResponse.Comment,
			CreatedAt:    timestamppb.New(reviewResponse.CreatedAt),
		},
	}, nil
}

func (s *server) UpdateReview(ctx context.Context, req *pb.UpdateReviewRequest) (*pb.UpdateReviewResponse, error) {
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		slog.Error("Failed to extract user from context", "error", err)
		return nil, mapToGRPCError(err)
	}
	user_id, err := parseInt32(claims.UserID)
	if err != nil {
		slog.Error("Invalid user ID in token", "userID", claims.UserID, "error", err)
		return nil, mapToGRPCError(err)
	}
	slog.Info("Received UpdateReview request", "reviewID", req.GetReviewId(), "userID", user_id)
	review := models.UpdateReview{
		ReviewID: req.GetReviewId(),
		UserID:   user_id,
	}
	if req.Comment != nil {
		comment := req.GetComment()
		review.Comment = &comment
	}
	if req.Rating != nil {
		rating := req.GetRating()
		review.Rating = &rating
	}
	updated, err := s.svc.UpdateReview(ctx, review)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("UpdateReview request successful", "reviewID", updated.ReviewID, "restaurantID", updated.RestaurantID, "userID", updated.UserID)
	return &pb.UpdateReviewResponse{
		Review: &pb.Review{
			ReviewId:     updated.ReviewID,
			RestaurantId: updated.RestaurantID,
			UserId:       updated.UserID,
			Rating:       updated.Rating,
			Comment:      updated.Comment,
			CreatedAt:    timestamppb.New(updated.CreatedAt),
		},
	}, nil
}

func (s *server) DeleteReview(ctx context.Context, req *pb.DeleteReviewRequest) (*pb.DeleteReviewResponse, error) {
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		slog.Error("Failed to extract user from context", "error", err)
		return nil, mapToGRPCError(err)
	}

	userID, err := parseInt32(claims.UserID)
	if err != nil {
		slog.Error("Invalid user ID in token", "userID", claims.UserID, "error", err)
		return nil, mapToGRPCError(err)
	}
	slog.Info("Received DeleteReview request", "reviewID", req.GetReviewId(), "userID", userID)

	err = s.svc.DeleteReview(ctx, models.DeleteReview{
		ReviewID: req.GetReviewId(),
		UserID:   userID,
	})
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("DeleteReview request successful", "reviewID", req.GetReviewId(), "userID", userID)
	return &pb.DeleteReviewResponse{}, nil
}

func (s *server) GetRestaurantStats(ctx context.Context, req *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error) {
	slog.Info("Received GetRestaurantStats request", "restaurantID", req.GetRestaurantId())
	stats, err := s.svc.GetRestaurantStats(ctx, req.GetRestaurantId())
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("GetRestaurantStats request successful", "restaurantID", req.GetRestaurantId(), "averageRating", stats.AverageRating, "reviewCount", stats.ReviewCount)
	return &pb.GetRestaurantStatsResponse{
		RestaurantStats: &pb.RestaurantStats{
			RestaurantId:  stats.RestaurantID,
			AverageRating: stats.AverageRating,
			ReviewCount:   stats.ReviewCount,
		},
	}, nil
}

func (app *App) RunServer() {
	cfg := config.Get(context.Background())
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	srv := &server{
		svc: app.Service,
	}

	pb.RegisterReviewServiceServer(s, srv)
	pb.RegisterReviewToRestaurantServiceServer(s, srv)

	slog.Info("Server running on", "address", addr)
	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve", "error", err)
		os.Exit(1)
	}
}

func ExtractUserFromContext(ctx context.Context) (*Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return nil, models.ErrForbidden
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")

	claims := &Claims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims)

	if err != nil {
		slog.Error("failed to parse token", "error", err)
		return nil, models.ErrUnauthenticated
	}

	slog.Info("USER INFO", "user_id", claims.UserID, "username", claims.Username, "token_type", claims.TokenType, "exp", claims.ExpiresAt)

	return claims, nil
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

func mapToGRPCError(err error) error {
	slog.Error("gRPC Operation Failed", "error", err)
	switch {
	case errors.Is(err, models.ErrInvalidRating), errors.Is(err, models.ErrInvalidComment), errors.Is(err, models.ErrInvalidRestaurantID),
		errors.Is(err, models.ErrInvalidUserID), errors.Is(err, models.ErrInvalidReviewID), errors.Is(err, models.ErrLimitTooLarge):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, models.ErrReviewAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, models.ErrRestaurantNotFound), errors.Is(err, models.ErrReviewNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, models.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, models.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, models.ErrRestaurantServiceUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, "Internal server error")
	}
}
