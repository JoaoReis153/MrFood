package service

import (
	"context"
	"errors"
	"strings"

	"MrFood/services/restaurant/internal/repository"
	models "MrFood/services/restaurant/pkg"
)

var (
	ErrInvalidRestaurant       = errors.New("invalid restaurant payload")
	ErrRestaurantAlreadyExists = errors.New("restaurant already exists")
	ErrForbidden               = errors.New("restaurant does not belong to user")
	ErrNotFound                = errors.New("restaurant not found")
	ErrInvalidCompareRequest   = errors.New("restaurant ids must be different")
)

type Service struct {
	repo restaurantRepository
}

type restaurantRepository interface {
	GetRestaurantByName(ctx context.Context, name string) (*models.Restaurant, error)
	CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error)
	GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error)
	UpdateRestaurant(ctx context.Context, restaurant *models.Restaurant) (*models.Restaurant, error)
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error) {
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

func (s *Service) UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int32) (*models.Restaurant, error) {
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

func (s *Service) GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error) {
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

	return restaurant, nil
}

func (s *Service) CompareRestaurants(ctx context.Context, id1, id2 int32) (*models.Restaurant, *models.Restaurant, error) {
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
