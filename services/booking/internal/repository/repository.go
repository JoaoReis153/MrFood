package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) CreateBooking(ctx context.Context, user_id, restaurant_id, people_count int, time_start, time_end time.Time) (int32, error) {
	query := `
		INSERT INTO booking (user_id, restaurant_id, time_start, time_end, people_count)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var booking_id int32

	err := r.DB.QueryRow(ctx, query, user_id, restaurant_id, time_start, time_end, people_count).Scan(&booking_id)

	if err != nil {
		slog.Error("Failed to create booking", "error", err)
		return 0, fmt.Errorf("Failed to create booking: %w", err)
	}

	return booking_id, nil
}

func (r *Repository) CheckBooking(ctx context.Context, restaurant_id int, time_start time.Time) (int32, error) {
	query := `
		SELECT COUNT(*)
		FROM bookings
		WHERE restaurant_id = $1
		AND time_start = $2;
	`
	var count int32

	err := r.DB.QueryRow(ctx, query, restaurant_id, time_start).Scan(&count)

	if err != nil {
		slog.Error("Failed to search for booking", "error", err)
		return 0, fmt.Errorf("Failed to create booking: %w", err)
	}

	return count, nil
}

func (r *Repository) GetSlots(ctx context.Context, restaurant_id int, time_start time.Time) (int32, int32, error) {
	query := `
		SELECT max_slots, current_slots
		FROM restaurant_slots
		WHERE restaurant_id = $1
		AND time_start = $2;
	`

	var max_slots, current_slots int32

	err := r.DB.QueryRow(ctx, query, restaurant_id, time_start).Scan(&max_slots, current_slots)

	if err != nil {
		slog.Error("Failed to search for slots", "error", err)
		return 0, 0, fmt.Errorf("Failed to create slots: %w", err)
	}

	return max_slots, current_slots, nil
}
