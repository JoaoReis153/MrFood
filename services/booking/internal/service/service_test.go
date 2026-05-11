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

// -----------------------------
// Mock Restaurant Client
// -----------------------------
type mockGRPCClient struct{}

func (m *mockGRPCClient) GetWorkingHours(ctx context.Context, req *pb.WorkingHoursRequest, opts ...grpc.CallOption) (*pb.WorkingHoursResponse, error) {
	return &pb.WorkingHoursResponse{
		RestaurantId: req.RestaurantId,
		TimeStart:    timestamppb.New(time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)),
		TimeEnd:      timestamppb.New(time.Date(2026, 3, 28, 18, 0, 0, 0, time.UTC)),
	}, nil
}

// -----------------------------
// Mock Payment Client
// -----------------------------
type mockPaymentClient struct{}

func (m *mockPaymentClient) MakePayment(ctx context.Context, req *pb.PaymentRequest, opts ...grpc.CallOption) (*pb.PaymentResponse, error) {
	return &pb.PaymentResponse{
		ReceiptId: 99,
	}, nil
}

// -----------------------------
// Mock Repo
// -----------------------------
type mockRepo struct {
	bookings map[int32]int64
}

func (m *mockRepo) CreateBooking(ctx context.Context, booking *models.Booking) (int32, error) {
	if booking.PeopleCount > MAX_SLOTS {
		return 0, ErrInvalidBooking
	}
	return 42, nil
}

func (m *mockRepo) DeleteBooking(ctx context.Context, req *models.DeleteBooking) error {
	if m.bookings == nil {
		m.bookings = map[int32]int64{
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

// -----------------------------
// CreateBooking Tests
// -----------------------------
func TestCreateBooking_EdgeCases(t *testing.T) {
	service := New(&mockRepo{}, &mockGRPCClient{}, &mockPaymentClient{})

	tests := []struct {
		name        string
		booking     *models.Booking
		expectErr   error
		expectID    int32
		expectRecID int32
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
				UserEmail:    "test@test.com",
				RestaurantID: 1,
				TimeStart:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				PeopleCount:  2,
			},
			expectErr:   nil,
			expectID:    42,
			expectRecID: 99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, receiptID, err := service.CreateBooking(context.Background(), tt.booking)

			if !errors.Is(err, tt.expectErr) {
				t.Fatalf("expected error %v, got %v", tt.expectErr, err)
			}

			if tt.expectErr == nil {
				if id != tt.expectID {
					t.Fatalf("expected booking id %d, got %d", tt.expectID, id)
				}
				if receiptID != tt.expectRecID {
					t.Fatalf("expected receipt id %d, got %d", tt.expectRecID, receiptID)
				}
			}
		})
	}
}

// -----------------------------
// DeleteBooking Tests
// -----------------------------
func TestDeleteBooking(t *testing.T) {
	service := New(&mockRepo{}, &mockGRPCClient{}, &mockPaymentClient{})

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
			BookingID: 999,
		})
		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound, got %v", err)
		}
	})

	t.Run("Wrong user", func(t *testing.T) {
		service.repo.(*mockRepo).bookings = map[int32]int64{1: 1}

		err := service.DeleteBooking(context.Background(), &models.DeleteBooking{
			UserID:    2,
			BookingID: 1,
		})
		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound for wrong user, got %v", err)
		}
	})
}
