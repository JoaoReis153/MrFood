package service

import (
	"context"
	"errors"
	"testing"

	"MrFood/services/restaurant/internal/repository"
	models "MrFood/services/restaurant/pkg"
)

type mockRepo struct {
	getByNameFn func(context.Context, string) (*models.Restaurant, error)
	createFn    func(context.Context, *models.Restaurant) (int32, error)
	getByIDFn   func(context.Context, int32) (*models.Restaurant, error)
	updateFn    func(context.Context, *models.Restaurant) (*models.Restaurant, error)
}

func (m *mockRepo) GetRestaurantByName(ctx context.Context, name string) (*models.Restaurant, error) {
	if m.getByNameFn == nil {
		return nil, repository.ErrRestaurantNotFound
	}
	return m.getByNameFn(ctx, name)
}

func (m *mockRepo) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error) {
	if m.createFn == nil {
		return 0, nil
	}
	return m.createFn(ctx, restaurant)
}

func (m *mockRepo) GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error) {
	if m.getByIDFn == nil {
		return nil, repository.ErrRestaurantNotFound
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockRepo) UpdateRestaurant(ctx context.Context, restaurant *models.Restaurant) (*models.Restaurant, error) {
	if m.updateFn == nil {
		return restaurant, nil
	}
	return m.updateFn(ctx, restaurant)
}

func TestCreateRestaurantRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name       string
		restaurant *models.Restaurant
	}{
		{name: "nil payload", restaurant: nil},
		{name: "missing owner", restaurant: &models.Restaurant{Name: "R1", OwnerID: 0}},
		{name: "missing name", restaurant: &models.Restaurant{Name: " ", OwnerID: 1}},
		{name: "negative max slots", restaurant: &models.Restaurant{Name: "R1", OwnerID: 1, MaxSlots: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{repo: &mockRepo{}}

			_, err := svc.CreateRestaurant(context.Background(), tt.restaurant)
			if !errors.Is(err, ErrInvalidRestaurant) {
				t.Fatalf("expected ErrInvalidRestaurant, got %v", err)
			}
		})
	}
}

func TestCreateRestaurantDuplicateName(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByNameFn: func(context.Context, string) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 10}, nil
		},
	}}

	_, err := svc.CreateRestaurant(context.Background(), &models.Restaurant{Name: "R1", OwnerID: 5})
	if !errors.Is(err, ErrRestaurantAlreadyExists) {
		t.Fatalf("expected ErrRestaurantAlreadyExists, got %v", err)
	}
}

func TestCreateRestaurantSuccess(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByNameFn: func(context.Context, string) (*models.Restaurant, error) {
			return nil, repository.ErrRestaurantNotFound
		},
		createFn: func(_ context.Context, restaurant *models.Restaurant) (int32, error) {
			if restaurant.Name != "R1" {
				t.Fatalf("expected name R1, got %q", restaurant.Name)
			}
			return 42, nil
		},
	}}

	id, err := svc.CreateRestaurant(context.Background(), &models.Restaurant{Name: "R1", OwnerID: 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected created id 42, got %d", id)
	}
}

func TestUpdateRestaurantRejectsNonOwner(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByIDFn: func(context.Context, int32) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 7, OwnerID: 100, Name: "R1"}, nil
		},
	}}

	_, err := svc.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 7, Name: "new"}, 999)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdateRestaurantMapsNotFound(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByIDFn: func(context.Context, int32) (*models.Restaurant, error) {
			return nil, repository.ErrRestaurantNotFound
		},
	}}

	_, err := svc.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 7}, 100)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateRestaurantDuplicateRename(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByIDFn: func(context.Context, int32) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 7, OwnerID: 100, Name: "Old Name"}, nil
		},
		getByNameFn: func(context.Context, string) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 8}, nil
		},
	}}

	_, err := svc.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 7, Name: "Taken Name"}, 100)
	if !errors.Is(err, ErrRestaurantAlreadyExists) {
		t.Fatalf("expected ErrRestaurantAlreadyExists, got %v", err)
	}
}

func TestUpdateRestaurantSuccessPreservesOwner(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByIDFn: func(context.Context, int32) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 7, OwnerID: 100, Name: "Old Name"}, nil
		},
		getByNameFn: func(context.Context, string) (*models.Restaurant, error) {
			return nil, repository.ErrRestaurantNotFound
		},
		updateFn: func(_ context.Context, restaurant *models.Restaurant) (*models.Restaurant, error) {
			if restaurant.OwnerID != 100 {
				t.Fatalf("expected owner to be preserved as 100, got %d", restaurant.OwnerID)
			}
			return restaurant, nil
		},
	}}

	updated, err := svc.UpdateRestaurant(context.Background(), &models.Restaurant{ID: 7, Name: "New Name"}, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("expected updated name New Name, got %q", updated.Name)
	}
}

func TestCompareRestaurantsRejectsEqualIDs(t *testing.T) {
	svc := &Service{repo: &mockRepo{}}

	_, _, err := svc.CompareRestaurants(context.Background(), 4, 4)
	if !errors.Is(err, ErrInvalidCompareRequest) {
		t.Fatalf("expected ErrInvalidCompareRequest, got %v", err)
	}
}

func TestCompareRestaurantsSuccess(t *testing.T) {
	svc := &Service{repo: &mockRepo{
		getByIDFn: func(_ context.Context, id int32) (*models.Restaurant, error) {
			return &models.Restaurant{ID: id, Name: "R"}, nil
		},
	}}

	r1, r2, err := svc.CompareRestaurants(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.ID != 1 || r2.ID != 2 {
		t.Fatalf("expected ids 1 and 2, got %d and %d", r1.ID, r2.ID)
	}
}
