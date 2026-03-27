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

const MAX_SLOTS int = 15

type Service struct {
	repo   *repository.Repository
	Client pb.RestaurantServiceClient
}

func New(repo *repository.Repository, client pb.RestaurantServiceClient) *Service {
	return &Service{repo: repo, Client: client}
}

func (s *Service) CreateBooking(ctx context.Context, booking *models.Booking, working_hours *models.WorkingHours) (*models.Booking, error) {
	// truncate start hour to minute 00 or 30
	booking.TimeStart = booking.TimeStart.UTC().Truncate(30 * time.Minute)

	if booking.TimeStart.Compare(working_hours.TimeStart) < 0 {
		slog.Error("Booking time is not valid: must be within working hours", "booking_hour_start", booking.TimeStart, "working_hour_start", working_hours.TimeStart)
		return nil, status.Error(codes.InvalidArgument, "invalid booking start time")
	}

	// check if booking already exists
	booking_exists, err := s.repo.CheckBooking(ctx, int(booking.UserID), int(booking.RestaurantID), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if booking_exists == 1 {
		slog.Error("Booking already exists")
		return nil, status.Error(codes.InvalidArgument, "booking already exists")
	}

	// check if there are sufficient available slots
	exists, max_slots, current_slots, err := s.repo.GetSlots(ctx, int(booking.RestaurantID), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if exists {
		if booking.PeopleCount > max_slots-current_slots {
			slog.Error("Not enough slots", "available_slots", max_slots-current_slots, "people_count", booking.PeopleCount)
			return nil, status.Error(codes.InvalidArgument, "not enough slots")
		}
	} else {
		if int(booking.PeopleCount) > MAX_SLOTS {
			slog.Error("Not enough slots", "max_slots", MAX_SLOTS, "people_count", booking.PeopleCount)
			return nil, status.Error(codes.InvalidArgument, "not enough slots")
		}
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

	if !exists {
		s.repo.CreateSlots(ctx, int(booking.UserID), int(booking.RestaurantID), MAX_SLOTS, int(booking.PeopleCount), booking.TimeStart, time_end)
	} else {
		s.repo.UpdateSlots(ctx, int(booking.RestaurantID), int(current_slots+booking.PeopleCount), booking.TimeStart)
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

func (s *Service) DeleteBooking(ctx context.Context, booking *models.Booking) error {
	booking.TimeStart = booking.TimeStart.UTC().Truncate(30 * time.Minute)

	people_count, err := s.repo.DeleteBooking(ctx, int(booking.UserID), int(booking.RestaurantID), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return status.Error(codes.Internal, err.Error())
	}

	_, _, current_slots, err := s.repo.GetSlots(ctx, int(booking.RestaurantID), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return status.Error(codes.Internal, err.Error())
	}

	newSlots := current_slots - people_count
	if newSlots < 0 {
		newSlots = 0
	}

	err = s.repo.UpdateSlots(ctx, int(booking.RestaurantID), int(newSlots), booking.TimeStart)

	if err != nil {
		slog.Error("Internal database error")
		return status.Error(codes.Internal, err.Error())
	}

	return nil
}
