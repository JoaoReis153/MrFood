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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
)

type mockRestaurantService struct {
	getByIDFn         func(context.Context, int64) (*models.Restaurant, error)
	getIDFn           func(context.Context, int64) (int64, error)
	createFn          func(context.Context, *models.Restaurant) (int64, error)
	updateFn          func(context.Context, *models.Restaurant, int64) (*models.Restaurant, error)
	compareFn         func(context.Context, int64, int64) (*models.Restaurant, *models.Restaurant, error)
	getWorkingHoursFn func(context.Context, int64, time.Time) (*models.WorkingHoursResponse, error)
}

type fakeReviewRPCClient struct {
	getStatsFn func(context.Context, *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error)
}

func (f *fakeReviewRPCClient) GetRestaurantStats(ctx context.Context, in *pb.GetRestaurantStatsRequest, _ ...grpc.CallOption) (*pb.GetRestaurantStatsResponse, error) {
	if f.getStatsFn == nil {
		return nil, nil
	}
	return f.getStatsFn(ctx, in)
}

func authContext(t *testing.T, userID, username string) context.Context {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"username":   username,
		"token_type": "access",
	}).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("failed creating token: %v", err)
	}

	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(context.Background(), md)
}

func (m *mockRestaurantService) GetRestaurantByID(ctx context.Context, id int64) (*models.Restaurant, error) {
	if m.getByIDFn == nil {
		return nil, errors.New("not configured")
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockRestaurantService) GetRestaurantID(ctx context.Context, id int64) (int64, error) {
	if m.getIDFn == nil {
		return 0, errors.New("not configured")
	}
	return m.getIDFn(ctx, id)
}

func (m *mockRestaurantService) CreateRestaurant(ctx context.Context, restaurant *models.Restaurant) (int64, error) {
	if m.createFn == nil {
		return 0, errors.New("not configured")
	}
	return m.createFn(ctx, restaurant)
}

func (m *mockRestaurantService) UpdateRestaurant(ctx context.Context, changes *models.Restaurant, requesterOwnerID int64) (*models.Restaurant, error) {
	if m.updateFn == nil {
		return nil, errors.New("not configured")
	}
	return m.updateFn(ctx, changes, requesterOwnerID)
}

func (m *mockRestaurantService) CompareRestaurants(ctx context.Context, id1, id2 int64) (*models.Restaurant, *models.Restaurant, error) {
	if m.compareFn == nil {
		return nil, nil, errors.New("not configured")
	}
	return m.compareFn(ctx, id1, id2)
}

func (m *mockRestaurantService) GetWorkingHours(ctx context.Context, restaurantID int64, timeStart time.Time) (*models.WorkingHoursResponse, error) {
	if m.getWorkingHoursFn == nil {
		return nil, errors.New("not configured")
	}
	return m.getWorkingHoursFn(ctx, restaurantID, timeStart)
}

func TestGetRestaurantDetailsSuccess(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{
		getByIDFn: func(context.Context, int64) (*models.Restaurant, error) {
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
		getIDFn: func(context.Context, int64) (int64, error) {
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
		getIDFn: func(context.Context, int64) (int64, error) {
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
		getWorkingHoursFn: func(context.Context, int64, time.Time) (*models.WorkingHoursResponse, error) {
			return &models.WorkingHoursResponse{TimeStart: start, TimeEnd: end, MaxSlots: 40}, nil
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
	if resp.GetMaxSlots() != 40 {
		t.Fatalf("expected max slots 40, got %d", resp.GetMaxSlots())
	}
}

func TestGetWorkingHoursWithoutTimestampUsesZeroTime(t *testing.T) {
	called := false
	srv := &server{restaurantService: &mockRestaurantService{
		getWorkingHoursFn: func(_ context.Context, _ int64, ts time.Time) (*models.WorkingHoursResponse, error) {
			called = true
			if !ts.IsZero() {
				t.Fatalf("expected zero timestamp, got %s", ts)
			}
			now := time.Now().UTC()
			return &models.WorkingHoursResponse{TimeStart: now, TimeEnd: now}, nil
		},
	}}

	_, err := srv.GetWorkingHours(context.Background(), &pb.WorkingHoursRequest{RestaurantId: 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected service call")
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
	rating := 4.6
	count := int32(19)
	model := &models.Restaurant{
		ID:            1,
		Name:          "Sushi",
		MediaURL:      "https://example.com/image.jpg",
		OpeningTime:   "09:00:00",
		ClosingTime:   "17:00:00",
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
	if pbModel.GetOpeningTime() != "09:00:00" || pbModel.GetClosingTime() != "17:00:00" {
		t.Fatalf("expected opening/closing time to be mapped, got open=%s close=%s", pbModel.GetOpeningTime(), pbModel.GetClosingTime())
	}
	if pbModel.GetAverageRating() != 4.6 || pbModel.GetReviewCount() != 19 {
		t.Fatalf("expected stats to be mapped, got rating=%v count=%v", pbModel.GetAverageRating(), pbModel.GetReviewCount())
	}

	model.AverageRating = nil
	model.ReviewCount = nil
	pbModel = modelToPB(model)
	if pbModel.GetAverageRating() != 0 || pbModel.GetReviewCount() != 0 {
		t.Fatalf("expected zero-value stats, got rating=%v count=%v", pbModel.GetAverageRating(), pbModel.GetReviewCount())
	}
}

func TestUUIDToInt64(t *testing.T) {
	uuid := "4f774104-1234-5678-abcd-ef0123456789"
	id := uuidToInt64(uuid)
	if id <= 0 {
		t.Fatalf("expected positive int64, got %d", id)
	}
	// Must be deterministic
	if uuidToInt64(uuid) != id {
		t.Fatal("uuidToInt64 is not deterministic")
	}
	// High bit cleared → always non-negative
	for _, s := range []string{"", "abc", uuid} {
		if v := uuidToInt64(s); v < 0 {
			t.Fatalf("expected non-negative for %q, got %d", s, v)
		}
	}
}

func TestExtractUserFromContext(t *testing.T) {
	if _, err := ExtractUserFromContext(context.Background()); err == nil {
		t.Fatal("expected metadata error")
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad-token"))
	if _, err := ExtractUserFromContext(ctx); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated for bad token, got %v", err)
	}

	ctx = authContext(t, "0", "mario")
	if _, err := ExtractUserFromContext(ctx); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated for invalid user id, got %v", err)
	}

	ctx = authContext(t, "7", "mario")
	user, err := ExtractUserFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.UserID != 7 || user.Username != "mario" {
		t.Fatalf("unexpected user info: %+v", user)
	}
}

func TestCreateAndUpdateRestaurantSuccess(t *testing.T) {
	createCalled := false
	updateCalled := false
	srv := &server{restaurantService: &mockRestaurantService{
		createFn: func(_ context.Context, r *models.Restaurant) (int64, error) {
			createCalled = true
			if r.OwnerID != 5 || r.OwnerName != "owner" {
				t.Fatalf("unexpected owner in create: %+v", r)
			}
			if r.OpeningTime != "10:00:00" || r.ClosingTime != "18:00:00" {
				t.Fatalf("unexpected opening/closing in create: %+v", r)
			}
			return 99, nil
		},
		updateFn: func(_ context.Context, r *models.Restaurant, requesterOwnerID int64) (*models.Restaurant, error) {
			updateCalled = true
			if requesterOwnerID != 5 || r.OpeningTime != "11:00:00" || r.ClosingTime != "20:00:00" {
				t.Fatalf("unexpected update request: %+v owner=%d", r, requesterOwnerID)
			}
			return &models.Restaurant{ID: r.ID, Name: r.Name}, nil
		},
	}}

	ctx := authContext(t, "5", "owner")
	createResp, err := srv.CreateRestaurant(ctx, &pb.CreateRestaurantRequest{Name: "Nori", Address: "A", OpeningTime: "10:00:00", ClosingTime: "18:00:00"})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if createResp.GetRestaurantId() != 99 || !createCalled {
		t.Fatalf("unexpected create response: %+v", createResp)
	}

	_, err = srv.UpdateRestaurant(ctx, &pb.UpdateRestaurantRequest{Id: 99, Name: "Nori 2", OpeningTime: "11:00:00", ClosingTime: "20:00:00"})
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if !updateCalled {
		t.Fatal("expected update call")
	}
}

func TestCreateRestaurantValidation(t *testing.T) {
	tests := []struct {
		name      string
		req       *pb.CreateRestaurantRequest
		wantCode  codes.Code
		createHit bool
	}{
		{
			name: "invalid latitude",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    91,
				Longitude:   1,
				OpeningTime: "12:00:00",
				ClosingTime: "18:00:00",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid longitude",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    45,
				Longitude:   -181,
				OpeningTime: "12:00:00",
				ClosingTime: "18:00:00",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing opening time",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    45,
				Longitude:   1,
				ClosingTime: "18:00:00",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid opening time format",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    45,
				Longitude:   1,
				OpeningTime: "12:00",
				ClosingTime: "18:00:00",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "duplicate categories",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    45,
				Longitude:   1,
				OpeningTime: "12:00:00",
				ClosingTime: "18:00:00",
				Categories:  []string{"Sushi", " sushi "},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "valid opening and closing time",
			req: &pb.CreateRestaurantRequest{
				Name:        "Nori",
				Latitude:    45,
				Longitude:   1,
				OpeningTime: "12:00:00",
				ClosingTime: "18:00:00",
				Categories:  []string{"Sushi", "Ramen"},
			},
			wantCode:  codes.OK,
			createHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createCalled := false
			srv := &server{restaurantService: &mockRestaurantService{
				createFn: func(_ context.Context, _ *models.Restaurant) (int64, error) {
					createCalled = true
					return 1, nil
				},
			}}

			_, err := srv.CreateRestaurant(authContext(t, "5", "owner"), tt.req)
			if status.Code(err) != tt.wantCode {
				t.Fatalf("expected %s, got err=%v", tt.wantCode, err)
			}
			if createCalled != tt.createHit {
				t.Fatalf("expected create called=%v, got %v", tt.createHit, createCalled)
			}
		})
	}
}

func TestCompareRestaurantDetailsAndMapInternal(t *testing.T) {
	srv := &server{restaurantService: &mockRestaurantService{
		compareFn: func(context.Context, int64, int64) (*models.Restaurant, *models.Restaurant, error) {
			return &models.Restaurant{ID: 1}, &models.Restaurant{ID: 2}, nil
		},
	}}

	resp, err := srv.CompareRestaurantDetails(context.Background(), &pb.CompareRestaurantDetailsRequest{RestaurantId_1: 1, RestaurantId_2: 2})
	if err != nil {
		t.Fatalf("unexpected compare error: %v", err)
	}
	if resp.GetRestaurant1().GetId() != 1 || resp.GetRestaurant2().GetId() != 2 {
		t.Fatalf("unexpected compare response: %+v", resp)
	}

	if status.Code(mapServiceError(errors.New("boom"))) != codes.Internal {
		t.Fatal("expected internal mapping")
	}
}

func TestReviewStatsClientGetRestaurantStats(t *testing.T) {
	client := &reviewStatsClient{client: &fakeReviewRPCClient{getStatsFn: func(context.Context, *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error) {
		return &pb.GetRestaurantStatsResponse{RestaurantStats: &pb.RestaurantStats{RestaurantId: 7, AverageRating: 4.2, ReviewCount: 11}}, nil
	}}}

	stats, err := client.GetRestaurantStats(context.Background(), 7)
	if err != nil || stats == nil {
		t.Fatalf("expected stats, got stats=%+v err=%v", stats, err)
	}
	if stats.RestaurantID != 7 || stats.ReviewCount != 11 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	client = &reviewStatsClient{client: &fakeReviewRPCClient{getStatsFn: func(context.Context, *pb.GetRestaurantStatsRequest) (*pb.GetRestaurantStatsResponse, error) {
		return nil, status.Error(codes.Unavailable, "down")
	}}}

	stats, err = client.GetRestaurantStats(context.Background(), 7)
	if err != nil || stats != nil {
		t.Fatalf("expected graceful nil stats on unavailable, got stats=%+v err=%v", stats, err)
	}
}

func TestAppNewAndCloseAndInitPanic(t *testing.T) {
	instance := New()
	if instance == nil {
		t.Fatal("expected app instance")
	}

	instance.Close()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when DB is nil")
		}
	}()
	instance.InitDependencies()
}
