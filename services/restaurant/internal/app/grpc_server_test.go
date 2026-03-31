package app

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/restaurant/internal/api/grpc/pb"
	"MrFood/services/restaurant/internal/service"
	models "MrFood/services/restaurant/pkg"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockRestaurantService struct {
	getByIDFn         func(context.Context, int32) (*models.Restaurant, error)
	getIDFn           func(context.Context, int32) (int32, error)
	createFn          func(context.Context, *models.Restaurant) (int32, error)
	updateFn          func(context.Context, *models.Restaurant, int32) (*models.Restaurant, error)
	compareFn         func(context.Context, int32, int32) (*models.Restaurant, *models.Restaurant, error)
	getWorkingHoursFn func(context.Context, int32, time.Time) (*models.TimeRange, error)
}

func (m *mockRestaurantService) GetRestaurantByID(ctx context.Context, id int32) (*models.Restaurant, error) {
	if m.getByIDFn == nil {
		return nil, errors.New("not configured")
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockRestaurantService) GetRestaurantID(ctx context.Context, id int32) (int32, error) {
	if m.getIDFn == nil {
		return 0, errors.New("not configured")
	}
	return m.getIDFn(ctx, id)
}

func (m *mockRestaurantService) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int32, error) {
	if m.createFn == nil {
		return 0, errors.New("not configured")
	}
	return m.createFn(ctx, restaurant)
}

func (m *mockRestaurantService) UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int32) (*models.Restaurant, error) {
	if m.updateFn == nil {
		return nil, errors.New("not configured")
	}
	return m.updateFn(ctx, changes, requesterOwnerID)
}

func (m *mockRestaurantService) CompareRestaurants(ctx context.Context, id1, id2 int32) (*models.Restaurant, *models.Restaurant, error) {
	if m.compareFn == nil {
		return nil, nil, errors.New("not configured")
	}
	return m.compareFn(ctx, id1, id2)
}

func (m *mockRestaurantService) GetWorkingHours(ctx context.Context, restaurantID int32, timeStart time.Time) (*models.TimeRange, error) {
	if m.getWorkingHoursFn == nil {
		return nil, errors.New("not configured")
	}
	return m.getWorkingHoursFn(ctx, restaurantID, timeStart)
}

func TestGetRestaurantDetailsSuccess(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{
		getByIDFn: func(context.Context, int32) (*models.Restaurant, error) {
			return &models.Restaurant{ID: 3, Name: "Nori"}, nil
		},
	}}

	resp, err := srv.GetRestaurantDetails(context.Background(), &pb.GetRestaurantDetailsRequest{Id: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetRestaurant().GetId() != 3 || resp.GetRestaurant().GetName() != "Nori" {
		t.Fatalf("unexpected response: %+v", resp.GetRestaurant())
	}
}

func TestCreateRestaurantMissingMetadata(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{}}

	_, err := srv.CreateRestaurant(context.Background(), &pb.CreateRestaurantRequest{Name: "Nori"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestGetRestaurantIdSuccess(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{
		getIDFn: func(context.Context, int32) (int32, error) {
			return 3, nil
		},
	}}

	resp, err := srv.GetRestaurantId(context.Background(), &pb.GetRestaurantRequest{RestaurantId: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetRestaurantId() != 3 {
		t.Fatalf("expected restaurant id 3, got %d", resp.GetRestaurantId())
	}
}

func TestGetRestaurantIdMapsNotFound(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{
		getIDFn: func(context.Context, int32) (int32, error) {
			return 0, service.ErrNotFound
		},
	}}

	_, err := srv.GetRestaurantId(context.Background(), &pb.GetRestaurantRequest{RestaurantId: 99})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestGetWorkingHoursSuccess(t *testing.T) {
	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	end := start.Add(8 * time.Hour)

	srv := &server{restaurantService: &mockRestaurantService{
		getWorkingHoursFn: func(context.Context, int32, time.Time) (*models.TimeRange, error) {
			return &models.TimeRange{TimeStart: start, TimeEnd: end}, nil
		},
	}}

	resp, err := srv.GetWorkingHours(context.Background(), &pb.WorkingHoursRequest{
		RestaurantId: 9,
		TimeStart:    timestamppb.New(start),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := resp.GetTimeStart().AsTime(); !got.Equal(start) {
		t.Fatalf("expected %s, got %s", start, got)
	}
	if got := resp.GetTimeEnd().AsTime(); !got.Equal(end) {
		t.Fatalf("expected %s, got %s", end, got)
	}
}

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid", err: service.ErrInvalidRestaurant, code: codes.InvalidArgument},
		{name: "already exists", err: service.ErrRestaurantAlreadyExists, code: codes.AlreadyExists},
		{name: "forbidden", err: service.ErrForbidden, code: codes.PermissionDenied},
		{name: "not found", err: service.ErrNotFound, code: codes.NotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapServiceError(tt.err)
			if status.Code(mapped) != tt.code {
				t.Fatalf("expected %s, got %s", tt.code, status.Code(mapped))
			}
		})
	}
}

func TestModelToPBMapsMediaAndWorkingHours(t *testing.T) {
	timestamp := "2026-03-27T12:00:00Z"
	rating := 4.6
	count := int32(19)
	model := &models.Restaurant{
		ID:            1,
		Name:          "Sushi",
		MediaURL:      "https://example.com/image.jpg",
		WorkingHours:  []string{timestamp},
		AverageRating: &rating,
		ReviewCount:   &count,
	}

	pbModel := modelToPB(model)
	if pbModel.GetId() != 1 || pbModel.GetName() != "Sushi" {
		t.Fatalf("unexpected mapped model: %+v", pbModel)
	}
	if pbModel.GetMediaUrl() == "" {
		t.Fatal("expected media url to be mapped")
	}
	if len(pbModel.GetWorkingHours()) != 1 {
		t.Fatalf("expected one working hour, got %d", len(pbModel.GetWorkingHours()))
	}
	if pbModel.GetAverageRating() != 4.6 || pbModel.GetReviewCount() != 19 {
		t.Fatalf("expected stats to be mapped, got rating=%v count=%v", pbModel.GetAverageRating(), pbModel.GetReviewCount())
	}

	model.AverageRating = nil
	model.ReviewCount = nil
	pbModel = modelToPB(model)
	if pbModel.AverageRating != nil || pbModel.ReviewCount != nil {
		t.Fatalf("expected nil optional stats, got rating=%v count=%v", pbModel.AverageRating, pbModel.ReviewCount)
	}
}
