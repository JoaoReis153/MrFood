package service

import (
	"MrFood/services/booking/internal/api/grpc/pb"
	models "MrFood/services/booking/pkg"
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrInvalidBooking       = errors.New("invalid booking payload")
	ErrBookingAlreadyExists = errors.New("booking already exists")
	ErrForbidden            = errors.New("booking does not belong to user")
	ErrBookingNotFound      = errors.New("booking not found")
	ErrFailedWHGet          = errors.New("failed to get working hours")
)

type BookingRepository interface {
	CreateBooking(ctx context.Context, booking *models.CreateBooking) (int32, error)
	DeleteBooking(ctx context.Context, delete_request *models.DeleteBooking) error
}

type Service struct {
	repo   BookingRepository
	client pb.RestaurantToBookingServiceClient
}

func New(repo BookingRepository, client pb.RestaurantToBookingServiceClient) *Service {
	return &Service{repo: repo, client: client}
}

func (s *Service) CreateBooking(ctx context.Context, booking *models.CreateBooking) (int32, error) {
	// truncate start hour to minute 00 or 30
	booking.TimeStart = booking.TimeStart.UTC().Truncate(30 * time.Minute)

	working_hours, err := s.getWorkingHours(ctx, booking.RestaurantID, booking.TimeStart)

	if err != nil {
		return 0, err
	}

	// check if people count is too high
	if booking.PeopleCount > working_hours.MaxSlots {
		slog.Error("Not enough slots", "people_count", booking.PeopleCount, "max_slots", working_hours.MaxSlots)
		return 0, ErrInvalidBooking
	}

	booking.MaxSlots = working_hours.MaxSlots
	slog.Info("max_slots", "max_slots", booking.MaxSlots)

	// check if start time is valid
	if booking.TimeStart.Before(working_hours.TimeStart) || booking.TimeStart.After(working_hours.TimeEnd) {
		slog.Error("Invalid booking time", "time_start", booking.TimeStart, "working_time_start", working_hours.TimeStart, "working_time_end", working_hours.TimeEnd)
		return 0, ErrInvalidBooking
	}

	var time_end = booking.TimeStart.Add(time.Hour)

	if time_end.After(working_hours.TimeEnd) {
		time_end = working_hours.TimeEnd
	}

	booking.TimeEnd = time_end

	booking_id, err := s.repo.CreateBooking(ctx, booking)

	if err != nil {
		return 0, err
	}

	return booking_id, nil
}

func (s *Service) DeleteBooking(ctx context.Context, delete_request *models.DeleteBooking) error {
	err := s.repo.DeleteBooking(ctx, delete_request)

	return err
}

func (s *Service) getWorkingHours(ctx context.Context, restaurantID int32, timeStart time.Time) (*models.WorkingHours, error) {
	res, err := s.client.GetWorkingHours(ctx, &pb.WorkingHoursRequest{
		RestaurantId: restaurantID,
		TimeStart:    timestamppb.New(timeStart),
	})
	if err != nil {
		return nil, err
	}

	return &models.WorkingHours{
		RestaurantID: res.GetRestaurantId(),
		TimeStart:    res.GetTimeStart().AsTime(),
		TimeEnd:      res.GetTimeEnd().AsTime(),
		MaxSlots:     res.GetMaxSlots(),
	}, nil
}
