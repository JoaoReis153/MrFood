package service

import (
	models "MrFood/services/review/pkg"
	"context"
	"errors"
	"testing"
)

type mockReviewRepo struct {
	getReviewsFn   func(ctx context.Context, restaurantID, page, limit int) ([]models.Review, int, error)
	createReviewFn func(ctx context.Context, review models.Review) (models.Review, error)
	updateReviewFn func(ctx context.Context, review models.UpdateReview) (models.Review, error)
	deleteReviewFn func(ctx context.Context, reviewID int32) error
}

func (m *mockReviewRepo) GetReviews(ctx context.Context, rID, p, l int) ([]models.Review, int, error) {
	return m.getReviewsFn(ctx, rID, p, l)
}
func (m *mockReviewRepo) CreateReview(ctx context.Context, r models.Review) (models.Review, error) {
	return m.createReviewFn(ctx, r)
}
func (m *mockReviewRepo) UpdateReview(ctx context.Context, r models.UpdateReview) (models.Review, error) {
	return m.updateReviewFn(ctx, r)
}
func (m *mockReviewRepo) DeleteReview(ctx context.Context, id int32) error {
	return m.deleteReviewFn(ctx, id)
}

type mockStatsRepo struct {
	getStatsFn func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error)
}

func (m *mockStatsRepo) GetRestaurantStats(ctx context.Context, id int32) (models.RestaurantStats, error) {
	return m.getStatsFn(ctx, id)
}

func TestGetReviews(t *testing.T) {
	t.Run("invalid restaurant ID", func(t *testing.T) {
		svc := &Service{}
		_, err := svc.GetReviews(context.Background(), 0, 1, 10)
		if !errors.Is(err, models.ErrInvalidRestaurantID) {
			t.Fatalf("expected ErrInvalidRestaurantID, got %v", err)
		}
	})

	t.Run("limit too large", func(t *testing.T) {
		svc := &Service{}
		_, err := svc.GetReviews(context.Background(), 1, 1, 101)
		if !errors.Is(err, models.ErrLimitTooLarge) {
			t.Fatalf("expected ErrLimitTooLarge, got %v", err)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		mockRepo := &mockReviewRepo{
			getReviewsFn: func(ctx context.Context, rID, p, l int) ([]models.Review, int, error) {
				return nil, 0, errors.New("query error")
			},
		}
		svc := New(mockRepo, nil)
		_, err := svc.GetReviews(context.Background(), 1, 1, 10)
		if err == nil || err.Error() != "query error" {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("success pagination calculation", func(t *testing.T) {
		mockRepo := &mockReviewRepo{
			getReviewsFn: func(ctx context.Context, rID, p, l int) ([]models.Review, int, error) {
				return make([]models.Review, 5), 15, nil
			},
		}
		svc := New(mockRepo, nil)
		res, err := svc.GetReviews(context.Background(), 1, 1, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Pagination.Pages != 3 {
			t.Errorf("expected 3 pages, got %d", res.Pagination.Pages)
		}
	})
}

func TestCreateReview(t *testing.T) {
	svc := &Service{}

	t.Run("invalid rating < 1", func(t *testing.T) {
		_, err := svc.CreateReview(context.Background(), models.Review{Rating: 0})
		if !errors.Is(err, models.ErrInvalidRating) {
			t.Fatalf("expected ErrInvalidRating, got %v", err)
		}
	})

	t.Run("invalid comment empty", func(t *testing.T) {
		_, err := svc.CreateReview(context.Background(), models.Review{Rating: 5, Comment: ""})
		if !errors.Is(err, models.ErrInvalidComment) {
			t.Fatalf("expected ErrInvalidComment, got %v", err)
		}
	})

	t.Run("invalid restaurant ID", func(t *testing.T) {
		_, err := svc.CreateReview(context.Background(), models.Review{Rating: 5, Comment: "Boa", RestaurantID: 0})
		if !errors.Is(err, models.ErrInvalidRestaurantID) {
			t.Fatalf("expected ErrInvalidRestaurantID, got %v", err)
		}
	})

	t.Run("invalid user ID", func(t *testing.T) {
		_, err := svc.CreateReview(context.Background(), models.Review{Rating: 5, Comment: "Boa", RestaurantID: 1, UserID: 0})
		if !errors.Is(err, models.ErrInvalidUserID) {
			t.Fatalf("expected ErrInvalidUserID, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockReviewRepo{
			createReviewFn: func(ctx context.Context, r models.Review) (models.Review, error) {
				r.ReviewID = 123
				return r, nil
			},
		}
		s := New(mockRepo, nil)
		res, err := s.CreateReview(context.Background(), models.Review{Rating: 5, Comment: "OK", RestaurantID: 1, UserID: 1})
		if err != nil || res.ReviewID != 123 {
			t.Fatalf("failed to create: %v", err)
		}
	})
}

func TestUpdateReview(t *testing.T) {
	svc := &Service{}

	t.Run("invalid review ID", func(t *testing.T) {
		_, err := svc.UpdateReview(context.Background(), models.UpdateReview{ReviewID: 0})
		if !errors.Is(err, models.ErrInvalidReviewID) {
			t.Fatalf("expected ErrInvalidReviewID, got %v", err)
		}
	})

	t.Run("invalid rating in update", func(t *testing.T) {
		rating := int32(6)
		_, err := svc.UpdateReview(context.Background(), models.UpdateReview{ReviewID: 1, Rating: &rating})
		if !errors.Is(err, models.ErrInvalidRating) {
			t.Fatalf("expected ErrInvalidRating, got %v", err)
		}
	})

	t.Run("invalid comment in update", func(t *testing.T) {
		comment := ""
		_, err := svc.UpdateReview(context.Background(), models.UpdateReview{ReviewID: 1, Comment: &comment})
		if !errors.Is(err, models.ErrInvalidComment) {
			t.Fatalf("expected ErrInvalidComment, got %v", err)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		mockRepo := &mockReviewRepo{
			updateReviewFn: func(ctx context.Context, r models.UpdateReview) (models.Review, error) {
				return models.Review{}, errors.New("update fail")
			},
		}
		s := New(mockRepo, nil)
		_, err := s.UpdateReview(context.Background(), models.UpdateReview{ReviewID: 1})
		if err == nil || err.Error() != "update fail" {
			t.Fatalf("expected update fail, got %v", err)
		}
	})
}

func TestDeleteReview(t *testing.T) {
	t.Run("invalid ID", func(t *testing.T) {
		svc := &Service{}
		err := svc.DeleteReview(context.Background(), 0)
		if !errors.Is(err, models.ErrInvalidReviewID) {
			t.Fatalf("expected ErrInvalidReviewID")
		}
	})

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockReviewRepo{
			deleteReviewFn: func(ctx context.Context, id int32) error {
				return nil
			},
		}
		svc := New(mockRepo, nil)
		if err := svc.DeleteReview(context.Background(), 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetRestaurantStats(t *testing.T) {
	t.Run("invalid restaurant ID", func(t *testing.T) {
		svc := &Service{}
		_, err := svc.GetRestaurantStats(context.Background(), -1)
		if !errors.Is(err, models.ErrInvalidRestaurantID) {
			t.Fatalf("expected ErrInvalidRestaurantID")
		}
	})

	t.Run("repo error", func(t *testing.T) {
		mockStats := &mockStatsRepo{
			getStatsFn: func(ctx context.Context, id int32) (models.RestaurantStats, error) {
				return models.RestaurantStats{}, errors.New("stats error")
			},
		}
		svc := New(nil, mockStats)
		_, err := svc.GetRestaurantStats(context.Background(), 1)
		if err == nil || err.Error() != "stats error" {
			t.Fatalf("expected stats error")
		}
	})
}
