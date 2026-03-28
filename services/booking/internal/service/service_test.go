package service

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/booking/internal/api/grpc/pb"
	models "MrFood/services/booking/pkg"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockGRPCClient struct{}

func (m *mockGRPCClient) GetWorkingHours(ctx context.Context, req *pb.WorkingHoursRequest, opts ...grpc.CallOption) (*pb.WorkingHoursResponse, error) {
	return &pb.WorkingHoursResponse{
		RestaurantId: req.RestaurantId,
		TimeStart:    timestamppb.New(time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)),  // 9:00 AM
		TimeEnd:      timestamppb.New(time.Date(2026, 3, 28, 18, 0, 0, 0, time.UTC)), // 6:00 PM
	}, nil
}

type mockRepo struct{}

func (m *mockRepo) CreateBooking(ctx context.Context, booking *models.Booking) (int32, error) {
	if booking.PeopleCount > MAX_SLOTS {
		return 0, ErrInvalidBooking
	}
	return 42, nil
}

func (m *mockRepo) DeleteBooking(ctx context.Context, userID, restaurantID int, start time.Time) error {
	if userID == 0 {
		return ErrBookingNotFound
	}
	return nil
}

func TestCreateBooking_EdgeCases(t *testing.T) {
	service := New(&mockRepo{}, &mockGRPCClient{})

	tests := []struct {
		name      string
		booking   *models.Booking
		expectErr error
		expectID  int32
	}{
		{
			name: "PeopleCount exceeds MAX_SLOTS",
			booking: &models.Booking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				PeopleCount:  16,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Time before working hours",
			booking: &models.Booking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Time after working hours",
			booking: &models.Booking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 19, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Successful booking",
			booking: &models.Booking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr: nil,
			expectID:  42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := service.CreateBooking(context.Background(), tt.booking)
			if !errors.Is(err, tt.expectErr) {
				t.Fatalf("expected error %v, got %v", tt.expectErr, err)
			}
			if tt.expectErr == nil && id != tt.expectID {
				t.Fatalf("expected booking id %d, got %d", tt.expectID, id)
			}
		})
	}
}

func TestDeleteBooking(t *testing.T) {
	service := New(&mockRepo{}, &mockGRPCClient{})

	err := service.DeleteBooking(context.Background(), &models.Booking{
		UserID:       1,
		RestaurantID: 1,
		TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = service.DeleteBooking(context.Background(), &models.Booking{
		UserID:       0,
		RestaurantID: 1,
		TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("expected ErrBookingNotFound, got %v", err)
	}
}
