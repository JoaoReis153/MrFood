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
		TimeStart:    timestamppb.New(time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)),
		TimeEnd:      timestamppb.New(time.Date(2026, 3, 28, 18, 0, 0, 0, time.UTC)),
		MaxSlots:     15,
	}, nil
}

type mockRepo struct {
	bookings map[int32]int32 // bookingID -> userID
}

func (m *mockRepo) CreateBooking(ctx context.Context, booking *models.CreateBooking) (int32, error) {
	return 42, nil
}

func (m *mockRepo) DeleteBooking(ctx context.Context, req *models.DeleteBooking) error {
	if m.bookings == nil {
		m.bookings = map[int32]int32{
			1: 1,
		}
	}

	userID, ok := m.bookings[req.BookingID]
	if !ok || userID != req.UserID {
		return ErrBookingNotFound
	}

	delete(m.bookings, req.BookingID)
	return nil
}

func TestCreateBooking_EdgeCases(t *testing.T) {
	service := New(&mockRepo{}, &mockGRPCClient{})

	tests := []struct {
		name      string
		booking   *models.CreateBooking
		expectErr error
		expectID  int32
	}{
		{
			name: "PeopleCount exceeds MAX_SLOTS",
			booking: &models.CreateBooking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				PeopleCount:  16,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Time before working hours",
			booking: &models.CreateBooking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Time after working hours",
			booking: &models.CreateBooking{
				UserID:       1,
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 19, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr: ErrInvalidBooking,
		},
		{
			name: "Successful booking",
			booking: &models.CreateBooking{
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

	t.Run("Successful deletion", func(t *testing.T) {
		err := service.DeleteBooking(context.Background(), &models.DeleteBooking{
			UserID:    1,
			BookingID: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Booking not found", func(t *testing.T) {
		err := service.DeleteBooking(context.Background(), &models.DeleteBooking{
			UserID:    1,
			BookingID: 999, // non-existent booking
		})
		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound, got %v", err)
		}
	})

	t.Run("Wrong user", func(t *testing.T) {
		// Recreate the booking to ensure it exists
		service.repo.(*mockRepo).bookings = map[int32]int32{1: 1}

		err := service.DeleteBooking(context.Background(), &models.DeleteBooking{
			UserID:    2, // wrong user
			BookingID: 1,
		})
		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound for wrong user, got %v", err)
		}
	})
}
