package service

import (
	models "MrFood/services/review/pkg"
	"errors"
)

type ReviewRepository interface {
	GetReviews(restaurantID, page, limit int) ([]models.Review, int, error)
	CreateReview(review models.Review) (int, error)
	UpdateReview(review models.Review) error
	DeleteReview(reviewID int) error
}

type Service struct {
	repo ReviewRepository
}

func New(repo ReviewRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetReviews(restaurantID, page, limit int) (models.ReviewsPage, error) {
	if limit <= 0 || limit > 100 {
		return models.ReviewsPage{}, errors.New("limit must be between 1 and 100")
	}
	if page <= 0 {
		return models.ReviewsPage{}, errors.New("page must be greater than 0")
	}

	reviews, total, err := s.repo.GetReviews(restaurantID, page, limit)
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

func (s *Service) CreateReview(review models.Review) (int, error) {
	if review.Rating < 1 || review.Rating > 5 {
		err := "rating must be between 1 and 5"
		return 0, errors.New(err)
	}
	if review.Comment != "" && len(review.Comment) > 100 {
		err := "comment must be less than 100 characters"
		return 0, errors.New(err)
	}
	return s.repo.CreateReview(review)
}

func (s *Service) UpdateReview(review models.Review) error {
	if review.Rating < 1 || review.Rating > 5 {
		err := "rating must be between 1 and 5"
		return errors.New(err)
	}
	if review.Comment != "" && len(review.Comment) > 100 {
		err := "comment must be less than 100 characters"
		return errors.New(err)
	}
	return s.repo.UpdateReview(review)
}

func (s *Service) DeleteReview(reviewID int) error {
	return s.repo.DeleteReview(reviewID)
}
