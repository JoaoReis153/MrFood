package service

import (
	models "MrFood/services/review/pkg"
	"context"
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
		return models.ReviewsPage{}, models.ErrInvalidRestaurantID
	}
	if limit > 100 {
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
		return models.Review{}, models.ErrInvalidRating
	}
	if review.Comment == "" || len(review.Comment) > 100 {
		return models.Review{}, models.ErrInvalidComment
	}
	if review.RestaurantID <= 0 {
		return models.Review{}, models.ErrInvalidRestaurantID
	}
	if review.UserID <= 0 {
		return models.Review{}, models.ErrInvalidUserID
	}
	return s.repo.CreateReview(ctx, review)
}

func (s *Service) UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error) {
	if review.Rating != nil && (*review.Rating < 1 || *review.Rating > 5) {
		return models.Review{}, models.ErrInvalidRating
	}
	if review.Comment != nil && (*review.Comment == "" || len(*review.Comment) > 100) {
		return models.Review{}, models.ErrInvalidComment
	}
	if review.ReviewID <= 0 {
		return models.Review{}, models.ErrInvalidReviewID
	}
	return s.repo.UpdateReview(ctx, review)
}

func (s *Service) DeleteReview(ctx context.Context, reviewID int) error {
	if reviewID <= 0 {
		return models.ErrInvalidReviewID
	}
	return s.repo.DeleteReview(ctx, reviewID)
}

func (s *Service) GetRestaurantStats(ctx context.Context, restaurantID int) (models.RestaurantStats, error) {
	if restaurantID <= 0 {
		return models.RestaurantStats{}, models.ErrInvalidRestaurantID
	}
	return s.statsRepo.GetRestaurantStats(ctx, restaurantID)
}
