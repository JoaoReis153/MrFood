package service

import (
	models "MrFood/services/review/pkg"
	"context"
	"log/slog"
)

type ReviewRepository interface {
	GetReviews(ctx context.Context, restaurantID, page, limit int) ([]models.Review, int, error)
	CreateReview(ctx context.Context, review models.Review) (models.Review, error)
	UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error)
	DeleteReview(ctx context.Context, reviewID int) error
}

type ReviewStatsRepository interface {
	GetRestaurantStats(ctx context.Context, restaurantID int) (models.RestaurantStats, error)
}

type Service struct {
	repo      ReviewRepository
	statsRepo ReviewStatsRepository
}

func New(repo ReviewRepository, statsRepo ReviewStatsRepository) *Service {
	return &Service{repo: repo, statsRepo: statsRepo}
}

func (s *Service) GetReviews(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error) {
	if restaurantID <= 0 {
		slog.Error("Restaurant ID is not valid: value must be a positive integer", "restaurantID", restaurantID)
		return models.ReviewsPage{}, models.ErrInvalidRestaurantID
	}
	if limit > 100 {
		slog.Error("Pagination limit is not valid: value must be between 1 and 100", "limit", limit)
		return models.ReviewsPage{}, models.ErrLimitTooLarge
	}

	reviews, total, err := s.repo.GetReviews(ctx, restaurantID, page, limit)
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
		slog.Error("Rating is not valid: value must be between 1 and 5", "rating", review.Rating)
		return models.Review{}, models.ErrInvalidRating
	}
	if review.Comment == "" || len(review.Comment) > 100 {
		slog.Error("Comment is not valid: value must be a non-empty string with a maximum length of 100 characters", "comment", review.Comment)
		return models.Review{}, models.ErrInvalidComment
	}
	if review.RestaurantID <= 0 {
		slog.Error("Restaurant ID is not valid: value must be a positive integer", "restaurantID", review.RestaurantID)
		return models.Review{}, models.ErrInvalidRestaurantID
	}
	if review.UserID <= 0 {
		slog.Error("User ID is not valid: value must be a positive integer", "userID", review.UserID)
		return models.Review{}, models.ErrInvalidUserID
	}
	return s.repo.CreateReview(ctx, review)
}

func (s *Service) UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error) {
	if review.Rating != nil && (*review.Rating < 1 || *review.Rating > 5) {
		slog.Error("Rating is not valid: value must be between 1 and 5", "rating", *review.Rating)
		return models.Review{}, models.ErrInvalidRating
	}
	if review.Comment != nil && (*review.Comment == "" || len(*review.Comment) > 100) {
		slog.Error("Comment is not valid: value must be a non-empty string with a maximum length of 100 characters", "comment", *review.Comment)
		return models.Review{}, models.ErrInvalidComment
	}
	if review.ReviewID <= 0 {
		slog.Error("Review ID is not valid: value must be a positive integer", "reviewID", review.ReviewID)
		return models.Review{}, models.ErrInvalidReviewID
	}
	return s.repo.UpdateReview(ctx, review)
}

func (s *Service) DeleteReview(ctx context.Context, reviewID int) error {
	if reviewID <= 0 {
		slog.Error("Review ID is not valid: value must be a positive integer", "reviewID", reviewID)
		return models.ErrInvalidReviewID
	}
	return s.repo.DeleteReview(ctx, reviewID)
}

func (s *Service) GetRestaurantStats(ctx context.Context, restaurantID int) (models.RestaurantStats, error) {
	if restaurantID <= 0 {
		slog.Error("Restaurant ID is not valid: value must be a positive integer", "restaurantID", restaurantID)
		return models.RestaurantStats{}, models.ErrInvalidRestaurantID
	}
	return s.statsRepo.GetRestaurantStats(ctx, restaurantID)
}
