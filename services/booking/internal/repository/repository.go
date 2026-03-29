package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	models "MrFood/services/booking/pkg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const MAX_SLOTS int32 = 15

var (
	ErrInvalidBooking       = errors.New("invalid booking arguments")
	ErrBookingAlreadyExists = errors.New("booking already exists")
	ErrBookingNotFound      = errors.New("booking not found")
)

type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	DB DB
}

func New(db DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) CreateBooking(ctx context.Context, booking *models.Booking) (int32, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// check if booking already exists
	query := `
		SELECT 1
		FROM booking
		WHERE restaurant_id = $1
		AND time_start = $2
		AND user_id = $3
		LIMIT 1;
	`

	var exists int32

	err = tx.QueryRow(ctx, query, booking.RestaurantID, booking.TimeStart, booking.UserID).Scan(&exists)

	if err != nil && err != pgx.ErrNoRows {
		return 0, err
	}

	if exists > 0 {
		return 0, ErrBookingAlreadyExists
	}

	// check if there are sufficient available slots
	query = `
		SELECT max_slots, current_slots
		FROM restaurant_slots
		WHERE restaurant_id = $1
		AND time_start = $2
		FOR UPDATE
	`

	var max_slots, current_slots int32

	err = tx.QueryRow(ctx, query, booking.RestaurantID, booking.TimeStart).Scan(&max_slots, &current_slots)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			max_slots = MAX_SLOTS
			current_slots = 0
		} else {
			return 0, err
		}
	} else if booking.PeopleCount > max_slots-current_slots {
		slog.Error("Not enough slots", "people_count", booking.PeopleCount, "available_slots", max_slots-current_slots)
		return 0, ErrInvalidBooking
	}

	// create booking
	query = `
		INSERT INTO booking (user_id, restaurant_id, time_start, time_end, people_count)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var booking_id int32

	err = tx.QueryRow(ctx, query, booking.UserID, booking.RestaurantID, booking.TimeStart, booking.TimeEnd, booking.PeopleCount).Scan(&booking_id)

	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return booking_id, nil
}

func (r *Repository) DeleteBooking(ctx context.Context, user_id, restaurant_id int, time_start time.Time) error {
	query := `
		DELETE FROM booking
		WHERE restaurant_id = $1
		AND time_start = $2
		AND user_id = $3;
	`

	cmdTag, err := r.DB.Exec(ctx, query, restaurant_id, time_start, user_id)

	if err != nil {
		return fmt.Errorf("failed to delete booking: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrBookingNotFound
	}

	return nil
}
