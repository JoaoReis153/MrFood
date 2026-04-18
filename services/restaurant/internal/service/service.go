package service

import (
	"MrFood/services/restaurant/internal/repository"
	models "MrFood/services/restaurant/pkg"
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

var (
	ErrInvalidRestaurant       = errors.New("invalid restaurant payload")
	ErrRestaurantAlreadyExists = errors.New("restaurant already exists")
	ErrForbidden               = errors.New("restaurant does not belong to user")
	ErrNotFound                = errors.New("restaurant not found")
	ErrInvalidCompareRequest   = errors.New("restaurant ids must be different")
)

type Service struct {
	repo        restaurantRepository
	reviewStats reviewStatsClient
}

type reviewStatsClient interface {
	GetRestaurantStats(ctx context.Context, restaurantID int64) (*models.RestaurantStats, error)
}

type restaurantRepository interface {
	GetRestaurantByName(ctx context.Context, name string) (*models.Restaurant, error)
	CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int64, error)
	GetRestaurantByID(ctx context.Context, id int64) (*models.Restaurant, error)
	GetRestaurantID(ctx context.Context, id int64) (int64, error)
	UpdateRestaurant(ctx context.Context, restaurant *models.Restaurant) (*models.Restaurant, error)
	GetWorkingHours(ctx context.Context, restaurantID int64, timeStart time.Time) (*models.WorkingHoursResponse, error)
}

func New(repo *repository.Repository, reviewStats reviewStatsClient) *Service {
	return &Service{repo: repo, reviewStats: reviewStats}
}

func (s *Service) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int64, error) {
	if restaurant == nil || restaurant.OwnerID <= 0 || strings.TrimSpace(restaurant.Name) == "" {
		return 0, ErrInvalidRestaurant
	}
	if restaurant.MaxSlots < 0 {
		return 0, ErrInvalidRestaurant
	}

	existing, err := s.repo.GetRestaurantByName(ctx, restaurant.Name)
	if err == nil && existing != nil {
		return 0, ErrRestaurantAlreadyExists
	}
	if err != nil && !errors.Is(err, repository.ErrRestaurantNotFound) {
		return 0, err
	}

	return s.repo.CreateRestaurant(ctx, restaurant)
}

func (s *Service) UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int64) (*models.Restaurant, error) {
	if changes == nil || changes.ID <= 0 || requesterOwnerID <= 0 {
		return nil, ErrInvalidRestaurant
	}

	existing, err := s.repo.GetRestaurantByID(ctx, changes.ID)
	if err != nil {
		if errors.Is(err, repository.ErrRestaurantNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if existing.OwnerID != requesterOwnerID {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(changes.Name) != "" && !strings.EqualFold(strings.TrimSpace(changes.Name), strings.TrimSpace(existing.Name)) {
		byName, err := s.repo.GetRestaurantByName(ctx, changes.Name)
		if err == nil && byName != nil && byName.ID != changes.ID {
			return nil, ErrRestaurantAlreadyExists
		}
		if err != nil && !errors.Is(err, repository.ErrRestaurantNotFound) {
			return nil, err
		}
	}

	if changes.MaxSlots < 0 {
		return nil, ErrInvalidRestaurant
	}

	changes.OwnerID = existing.OwnerID

	return s.repo.UpdateRestaurant(ctx, changes)
}

func (s *Service) GetRestaurantByID(ctx context.Context, id int64) (*models.Restaurant, error) {
	if id <= 0 {
		return nil, ErrInvalidRestaurant
	}

	restaurant, err := s.repo.GetRestaurantByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrRestaurantNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	slog.Info("got restaurant by id", "restaurant_id", id, "restaurant", restaurant)
	return s.enrichWithReviewStats(ctx, restaurant)
}

func (s *Service) GetRestaurantID(ctx context.Context, id int64) (int64, error) {
	if id <= 0 {
		return 0, ErrInvalidRestaurant
	}

	restaurantID, err := s.repo.GetRestaurantID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrRestaurantNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return restaurantID, nil
}

func (s *Service) CompareRestaurants(ctx context.Context, id1, id2 int64) (*models.Restaurant, *models.Restaurant, error) {
	if id1 <= 0 || id2 <= 0 {
		return nil, nil, ErrInvalidRestaurant
	}
	if id1 == id2 {
		return nil, nil, ErrInvalidCompareRequest
	}

	r1, err := s.GetRestaurantByID(ctx, id1)
	if err != nil {
		return nil, nil, err
	}

	r2, err := s.GetRestaurantByID(ctx, id2)
	if err != nil {
		return nil, nil, err
	}

	return r1, r2, nil
}

func (s *Service) GetWorkingHours(ctx context.Context, restaurantID int64, timeStart time.Time) (*models.WorkingHoursResponse, error) {
	if restaurantID <= 0 {
		return nil, ErrInvalidRestaurant
	}

	workingHours, err := s.repo.GetWorkingHours(ctx, restaurantID, timeStart)
	if err != nil {
		if errors.Is(err, repository.ErrRestaurantNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return workingHours, nil
}

func (s *Service) enrichWithReviewStats(ctx context.Context, restaurant *models.Restaurant) (*models.Restaurant, error) {
	if restaurant == nil {
		return nil, ErrNotFound
	}
	slog.Info("enriching restaurant with review stats", "restaurant_id", restaurant.ID, "review_stats_client_nil", s.reviewStats == nil)
	if s.reviewStats == nil {
		return restaurant, nil
	}
	slog.Debug("enriching restaurant with review stats not null", "restaurant_id", restaurant.ID)
	stats, err := s.reviewStats.GetRestaurantStats(ctx, restaurant.ID)
	slog.Debug("got review stats", "restaurant_id", restaurant.ID, "stats", stats, "error", err)
	if err != nil {
		return restaurant, nil
	}
	if stats == nil {
		return restaurant, nil
	}

	restaurant.AverageRating = &stats.AverageRating
	restaurant.ReviewCount = &stats.ReviewCount
	slog.Debug("enriched restaurant with review stats", "restaurant_id", restaurant.ID, "average_rating", restaurant.AverageRating, "review_count", restaurant.ReviewCount)
	return restaurant, nil
}
