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

func (r *Repository) CreateBooking(ctx context.Context, user_id, restaurant_id, people_count int, time_start time.Time) (int32, error) {
	query := `
		INSERT INTO booking (user_id, restaurant_id, people_count)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var booking_id int32

	err := r.DB.QueryRow(ctx, query, user_id, restaurant_id, people_count, restaurant_id).Scan(&booking_id)

	if err != nil {
		slog.Error("Failed to create booking", "error", err)
		return 0, fmt.Errorf("Failed to create booking: %w", err)
	}

	return booking_id, nil
}
