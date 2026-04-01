package app

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/service"
	models "MrFood/services/booking/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockService struct {
	createFn func(ctx context.Context, booking *models.Booking) (int32, error)
	deleteFn func(ctx context.Context, delete_request *models.DeleteBooking) error
}

func (m *mockService) CreateBooking(ctx context.Context, b *models.Booking) (int32, error) {
	return m.createFn(ctx, b)
}

func (m *mockService) DeleteBooking(ctx context.Context, b *models.DeleteBooking) error {
	return m.deleteFn(ctx, b)
}

// helper to create context with JWT
func ctxWithToken(userID string) context.Context {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
	})
	str, _ := token.SignedString([]byte("secret"))

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + str,
	})

	return metadata.NewIncomingContext(context.Background(), md)
}

func TestCreateBooking(t *testing.T) {

	now := time.Now()

	baseReq := &pb.CreateBookingRequest{
		RestaurantId: 1,
		TimeStart:    timestamppb.New(now),
		Quantity:     2,
	}

	t.Run("no metadata", func(t *testing.T) {
		s := &server{}

		_, err := s.CreateBooking(context.Background(), baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("no auth header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})

		s := &server{}

		_, err := s.CreateBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid user_id parse", func(t *testing.T) {
		ctx := ctxWithToken("invalid")

		s := &server{}

		_, err := s.CreateBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected parse error")
		}
	})

	t.Run("service returns ErrInvalidBooking", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				createFn: func(ctx context.Context, b *models.Booking) (int32, error) {
					return 0, service.ErrInvalidBooking
				},
			},
		}

		_, err := s.CreateBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("service returns ErrBookingAlreadyExists", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				createFn: func(ctx context.Context, b *models.Booking) (int32, error) {
					return 0, service.ErrBookingAlreadyExists
				},
			},
		}

		_, err := s.CreateBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				createFn: func(ctx context.Context, b *models.Booking) (int32, error) {
					if b.UserID != 1 {
						t.Fatalf("expected userID 1, got %d", b.UserID)
					}
					return 42, nil
				},
			},
		}

		res, err := s.CreateBooking(ctx, baseReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.BookingId != 42 {
			t.Fatalf("expected id 42, got %d", res.BookingId)
		}
	})
}

func TestDeleteBooking(t *testing.T) {

	baseReq := &pb.DeleteBookingRequest{
		BookingId: 1,
	}

	t.Run("no metadata", func(t *testing.T) {
		s := &server{}

		_, err := s.DeleteBooking(context.Background(), baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid user_id", func(t *testing.T) {
		ctx := ctxWithToken("bad")

		s := &server{}

		_, err := s.DeleteBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("service returns not found", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				deleteFn: func(ctx context.Context, b *models.DeleteBooking) error {
					return service.ErrBookingNotFound
				},
			},
		}

		_, err := s.DeleteBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("service error", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				deleteFn: func(ctx context.Context, b *models.DeleteBooking) error {
					return errors.New("some error")
				},
			},
		}

		_, err := s.DeleteBooking(ctx, baseReq)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx := ctxWithToken("1")

		s := &server{
			bookingService: &mockService{
				deleteFn: func(ctx context.Context, b *models.DeleteBooking) error {
					if b.UserID != 1 {
						t.Fatalf("expected userID 1, got %d", b.UserID)
					}
					return nil
				},
			},
		}

		_, err := s.DeleteBooking(ctx, baseReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
