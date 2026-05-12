package service

import (
	models "MrFood/services/review/pkg"
	"context"
	"log/slog"
	"strings"
)

type ReviewRepository interface {
	GetReviews(ctx context.Context, restaurantID int64, page, limit int) ([]models.Review, int, error)
	CreateReview(ctx context.Context, review models.Review) (models.Review, error)
	UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error)
	DeleteReview(ctx context.Context, reviewID int64, userID int64) error
	GetRestaurantStats(ctx context.Context, restaurantID int64) (models.RestaurantStats, error)
}

type RestaurantClient interface {
	GetRestaurant(ctx context.Context, restaurantID int64) (models.Restaurant, error)
}

type Service struct {
	repo             ReviewRepository
	restaurantClient RestaurantClient
}

func New(repo ReviewRepository, restaurantClient RestaurantClient) *Service {
	return &Service{repo: repo, restaurantClient: restaurantClient}
}

func (s *Service) GetReviews(ctx context.Context, restaurantID int64, page, limit int) (models.ReviewsPage, error) {
	if restaurantID <= 0 {
		slog.WarnContext(ctx, "invalid restaurant_id", "restaurant_id", restaurantID)
		return models.ReviewsPage{}, models.ErrInvalidRestaurantID
	}
	if limit > 100 {
		slog.WarnContext(ctx, "invalid pagination limit", "limit", limit)
		return models.ReviewsPage{}, models.ErrLimitTooLarge
	}

	restaurant, err := s.restaurantClient.GetRestaurant(ctx, restaurantID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get restaurant details", "restaurant_id", restaurantID, "error", err)
		return models.ReviewsPage{}, err
	}

	reviews, total, err := s.repo.GetReviews(ctx, restaurant.RestaurantID, page, limit)
	if err != nil {
		return models.ReviewsPage{}, err
	}

	return models.ReviewsPage{
		Reviews: reviews,
		Pagination: models.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
			Pages: (total + limit - 1) / limit,
		},
	}, nil
}

func (s *Service) CreateReview(ctx context.Context, review models.Review) (models.Review, error) {
	if review.Rating < 1 || review.Rating > 5 {
		slog.WarnContext(ctx, "invalid rating", "rating", review.Rating)
		return models.Review{}, models.ErrInvalidRating
	}
	if strings.TrimSpace(review.Comment) == "" || len(review.Comment) > 100 {
		slog.WarnContext(ctx, "invalid comment", "comment", review.Comment)
		return models.Review{}, models.ErrInvalidComment
	}
	if review.RestaurantID <= 0 {
		slog.WarnContext(ctx, "invalid restaurant_id", "restaurant_id", review.RestaurantID)
		return models.Review{}, models.ErrInvalidRestaurantID
	}
	if review.UserID <= 0 {
		slog.WarnContext(ctx, "invalid user_id", "user_id", review.UserID)
		return models.Review{}, models.ErrInvalidUserID
	}
	restaurant, err := s.restaurantClient.GetRestaurant(ctx, review.RestaurantID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get restaurant details", "restaurant_id", restaurant.RestaurantID, "error", err)
		return models.Review{}, err
	}

	return s.repo.CreateReview(ctx, review)
}

func (s *Service) UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error) {
	if review.Rating != nil && (*review.Rating < 1 || *review.Rating > 5) {
		slog.WarnContext(ctx, "invalid rating", "rating", *review.Rating)
		return models.Review{}, models.ErrInvalidRating
	}
	if review.Comment != nil && (strings.TrimSpace(*review.Comment) == "" || len(*review.Comment) > 100) {
		slog.WarnContext(ctx, "invalid comment", "comment", *review.Comment)
		return models.Review{}, models.ErrInvalidComment
	}
	if review.ReviewID <= 0 {
		slog.WarnContext(ctx, "invalid review_id", "review_id", review.ReviewID)
		return models.Review{}, models.ErrInvalidReviewID
	}
	return s.repo.UpdateReview(ctx, review)
}

func (s *Service) DeleteReview(ctx context.Context, deleteReq models.DeleteReview) error {
	if deleteReq.ReviewID <= 0 {
		slog.WarnContext(ctx, "invalid review_id", "review_id", deleteReq.ReviewID)
		return models.ErrInvalidReviewID
	}
	if deleteReq.UserID <= 0 {
		slog.WarnContext(ctx, "invalid user_id", "user_id", deleteReq.UserID)
		return models.ErrInvalidUserID
	}
	return s.repo.DeleteReview(ctx, deleteReq.ReviewID, deleteReq.UserID)
}

func (s *Service) GetRestaurantStats(ctx context.Context, restaurantID int64) (models.RestaurantStats, error) {
	if restaurantID <= 0 {
		slog.WarnContext(ctx, "invalid restaurant_id", "restaurant_id", restaurantID)
		return models.RestaurantStats{}, models.ErrInvalidRestaurantID
	}
	return s.repo.GetRestaurantStats(ctx, restaurantID)
}
