package service

import (
	"MrFood/services/booking/internal/api/grpc/pb"
	"MrFood/services/booking/internal/repository"
	models "MrFood/services/booking/pkg"
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	repo   *repository.Repository
	Client pb.RestaurantServiceClient
}

func New(repo *repository.Repository, client pb.RestaurantServiceClient) *Service {
	return &Service{repo: repo, Client: client}
}

func (s *Service) CreateBooking(ctx context.Context, booking *models.Booking, working_hours *models.WorkingHours) (*models.Booking, error) {
	// check if starting hour is valid
	if booking.TimeStart.Minute() != 0 || booking.TimeStart.Minute() != 30 {
		slog.Error("Booking time is not valid: minutes must be either 00 or 30")
		return nil, status.Error(codes.InvalidArgument, "invalid booking start time")
	}

	// check if booking already exists
	count, err := s.repo.CheckBooking(ctx, int(working_hours.RestaurantID), working_hours.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if count > 0 {
		slog.Error("Booking already exists", "count", count)
		return nil, status.Error(codes.InvalidArgument, "booking already exists")
	}

	// check if there are sufficient available slots
	max_slots, current_slots, err := s.repo.GetSlots(ctx, int(booking.RestaurantID), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return nil, status.Error(codes.Internal, err.Error())
	}

	available_slots := max_slots - current_slots

	if booking.PeopleCount > available_slots {
		slog.Error("Not enough slots", "available_slots", available_slots, "people_count", booking.PeopleCount)
		return nil, status.Error(codes.InvalidArgument, "not enough slots")
	}

	// proceed with booking
	time_end := booking.TimeStart.Add(time.Hour * time.Duration(1))

	if time_end.After(working_hours.TimeEnd) {
		time_end = working_hours.TimeEnd
	}

	booking_id, err := s.repo.CreateBooking(ctx, int(booking.UserID), int(booking.RestaurantID), int(booking.PeopleCount), booking.TimeStart, time_end)

	if err != nil {
		slog.Error("Internal database error")
		return nil, status.Error(codes.Internal, err.Error())
	}

	booking = &models.Booking{
		ID:           booking_id,
		UserID:       booking.UserID,
		RestaurantID: booking.RestaurantID,
		PeopleCount:  booking.PeopleCount,
		TimeStart:    booking.TimeStart,
		TimeEnd:      time_end,
	}

	return booking, nil
}
