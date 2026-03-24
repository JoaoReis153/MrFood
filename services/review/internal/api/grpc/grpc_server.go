package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"

	pb "MrFood/services/review/internal/api/grpc/pb"
	"MrFood/services/review/internal/service"
	models "MrFood/services/review/pkg"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedReviewServiceServer
	pb.UnimplementedReviewToDetailsServiceServer
	svc *service.Service
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
			ReviewId:     int32(r.ReviewID),
			RestaurantId: int32(r.RestaurantID),
			UserId:       int32(r.UserID),
			Rating:       int32(r.Rating),
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
	slog.Info("Received CreateReview request", "restaurantID", req.GetRestaurantId(), "userID", req.GetUserId(), "rating", req.GetRating())
	review := models.Review{
		RestaurantID: int(req.GetRestaurantId()),
		UserID:       int(req.GetUserId()),
		Rating:       int(req.GetRating()),
		Comment:      req.GetComment(),
	}
	review, err := s.svc.CreateReview(ctx, review)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("CreateReview request successful", "reviewID", review.ReviewID, "restaurantID", review.RestaurantID, "userID", review.UserID)
	return &pb.CreateReviewResponse{
		Review: &pb.Review{
			ReviewId:     int32(review.ReviewID),
			RestaurantId: int32(review.RestaurantID),
			UserId:       int32(review.UserID),
			Rating:       int32(review.Rating),
			Comment:      review.Comment,
			CreatedAt:    timestamppb.New(review.CreatedAt),
		},
	}, nil
}

func (s *server) UpdateReview(ctx context.Context, req *pb.UpdateReviewRequest) (*pb.UpdateReviewResponse, error) {
	slog.Info("Received UpdateReview request", "reviewID", req.GetReviewId())
	review := models.UpdateReview{
		ReviewID: int(req.GetReviewId()),
	}
	if req.Comment != nil {
		comment := req.GetComment()
		review.Comment = &comment
	}
	if req.Rating != nil {
		rating := int(req.GetRating())
		review.Rating = &rating
	}
	updated, err := s.svc.UpdateReview(ctx, review)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("UpdateReview request successful", "reviewID", updated.ReviewID, "restaurantID", updated.RestaurantID, "userID", updated.UserID)
	return &pb.UpdateReviewResponse{
		Review: &pb.Review{
			ReviewId:     int32(updated.ReviewID),
			RestaurantId: int32(updated.RestaurantID),
			UserId:       int32(updated.UserID),
			Rating:       int32(updated.Rating),
			Comment:      updated.Comment,
			CreatedAt:    timestamppb.New(updated.CreatedAt),
		},
	}, nil
}

func (s *server) DeleteReview(ctx context.Context, req *pb.DeleteReviewRequest) (*pb.DeleteReviewResponse, error) {
	slog.Info("Received DeleteReview request", "reviewID", req.GetReviewId())
	err := s.svc.DeleteReview(ctx, int(req.GetReviewId()))
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("DeleteReview request successful", "reviewID", req.GetReviewId())
	return &pb.DeleteReviewResponse{}, nil
}

func (s *server) GetRestaurantStats(ctx context.Context, req *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error) {
	slog.Info("Received GetRestaurantStats request", "restaurantID", req.GetRestaurantId())
	stats, err := s.svc.GetRestaurantStats(ctx, int(req.GetRestaurantId()))
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	slog.Info("GetRestaurantStats request successful", "restaurantID", req.GetRestaurantId(), "averageRating", stats.AverageRating, "reviewCount", stats.ReviewCount)
	return &pb.GetRestaurantStatsResponse{
		RestaurantStats: &pb.RestaurantStats{
			RestaurantId:  int32(stats.RestaurantID),
			AverageRating: float64(stats.AverageRating),
			ReviewCount:   int32(stats.ReviewCount),
		},
	}, nil
}

func RunServer(svc *service.Service) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	pb.RegisterReviewServiceServer(s, &server{svc: svc})
	pb.RegisterReviewToDetailsServiceServer(s, &server{svc: svc})
	reflection.Register(s)

	fmt.Println("Server running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatal(err)
	}
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
	default:
		return status.Error(codes.Internal, "Internal server error")
	}
}
