package service

import (
	"MrFood/services/booking/internal/api/grpc/pb"
	models "MrFood/services/booking/pkg"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const MAX_SLOTS int32 = 15

var (
	ErrInvalidBooking       = errors.New("invalid booking payload")
	ErrBookingAlreadyExists = errors.New("booking already exists")
	ErrForbidden            = errors.New("booking does not belong to user")
	ErrBookingNotFound      = errors.New("booking not found")
	ErrFailedWHGet          = errors.New("failed to get working hours")
)

type BookingRepository interface {
	CreateBooking(ctx context.Context, booking *models.Booking) (int32, error)
	DeleteBooking(ctx context.Context, delete_request *models.DeleteBooking) error
}

type Service struct {
	repo             BookingRepository
	restaurantClient pb.RestaurantToBookingServiceClient
	paymentClient    pb.PaymentCommandServiceClient
}

func New(repo BookingRepository, restaurantClient pb.RestaurantToBookingServiceClient, paymentClient pb.PaymentCommandServiceClient) *Service {
	return &Service{repo: repo, restaurantClient: restaurantClient, paymentClient: paymentClient}
}

func (s *Service) CreateBooking(ctx context.Context, booking *models.Booking) (int32, int32, error) {
	// check if people count is too high
	if booking.PeopleCount > MAX_SLOTS {
		slog.Error("Not enough slots", "people_count", booking.PeopleCount, "max_slots", MAX_SLOTS)
		return 0, 0, ErrInvalidBooking
	}

	// truncate start hour to minute 00 or 30
	booking.TimeStart = booking.TimeStart.UTC().Truncate(30 * time.Minute)

	working_hours, err := s.getWorkingHours(ctx, booking.RestaurantID, booking.TimeStart)

	if err != nil {
		return 0, 0, err
	}

	if booking.TimeStart.Before(working_hours.TimeStart) || booking.TimeStart.After(working_hours.TimeEnd) {
		slog.Error("Invalid booking time", "time_start", booking.TimeStart, "working_time_start", working_hours.TimeStart, "working_time_end", working_hours.TimeEnd)
		return 0, 0, ErrInvalidBooking
	}

	var time_end = booking.TimeStart.Add(time.Hour)

	if time_end.After(working_hours.TimeEnd) {
		time_end = working_hours.TimeEnd
	}

	booking.TimeEnd = time_end

	booking_id, err := s.repo.CreateBooking(ctx, booking)

	if err != nil {
		return 0, 0, err
	}

	amount := float32(booking.PeopleCount) * 10

	receipt_id, err := s.makePayment(ctx, &models.PaymentRequest{
		UserID:         booking.UserID,
		UserEmail:      booking.UserEmail,
		IdempotencyKey: GenerateIdempotencyKey(booking.UserID, amount, booking_id, "B"),
		Amount:         float32(amount),
		PaymentDescription: fmt.Sprintf("BOOKING %d FOR USER %d IN RESTAURANT %d FROM %s TO %s",
			booking_id, booking.UserID, booking.RestaurantID, FormatTime(booking.TimeStart), FormatTime(booking.TimeEnd)),
		PaymentType: "B",
	})

	return booking_id, receipt_id, nil
}

func (s *Service) DeleteBooking(ctx context.Context, delete_request *models.DeleteBooking) error {
	err := s.repo.DeleteBooking(ctx, delete_request)

	return err
}

func (s *Service) makePayment(ctx context.Context, req *models.PaymentRequest) (int32, error) {
	res, err := s.paymentClient.MakePayment(ctx, &pb.PaymentRequest{
		UserId:             req.UserID,
		UserEmail:          req.UserEmail,
		Amount:             req.Amount,
		IdempotencyKey:     req.IdempotencyKey,
		PaymentDescription: req.PaymentDescription,
		Type:               req.PaymentType,
	})

	if err != nil {
		slog.Error("failed to get receipt", "error", err)
		return 0, err
	}

	slog.Info("receipt id", "receipt_id", res.ReceiptId)
	return res.ReceiptId, nil
}

func (s *Service) getWorkingHours(ctx context.Context, restaurantID int64, timeStart time.Time) (*models.WorkingHours, error) {
	res, err := s.restaurantClient.GetWorkingHours(ctx, &pb.WorkingHoursRequest{
		RestaurantId: restaurantID,
		TimeStart:    timestamppb.New(timeStart),
	})
	if err != nil {
		return nil, err
	}

	return &models.WorkingHours{
		RestaurantID: res.RestaurantId,
		TimeStart:    res.TimeStart.AsTime(),
		TimeEnd:      res.TimeEnd.AsTime(),
	}, nil
}

func GenerateIdempotencyKey(userID int64, amount float32, bookingID int32, service string) string {
	data := fmt.Sprintf("%d:%f:%d:%s", userID, amount, bookingID, service)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func FormatTime(t time.Time) string {
	return t.UTC().Truncate(30 * time.Minute).Format("2006-01-02T15:04")
}
