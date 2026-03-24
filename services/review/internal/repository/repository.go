package repository

import (
	models "MrFood/services/review/pkg"
	"context"
	sql "database/sql"
	"errors"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetReviews(ctx context.Context, restaurantID, page, limit int) ([]models.Review, int, error) {
	offset := (page - 1) * limit
	var total int
	err := r.db.QueryRowContext(ctx,
		"SELECT review_count FROM restaurant_stats WHERE restaurant_id = $1",
		restaurantID).Scan(&total)

	if err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}
	if total == 0 {
		return []models.Review{}, 0, nil
	}

	rows, err := r.db.QueryContext(ctx, `
        SELECT review_id, restaurant_id, user_id, comment, rating, created_at 
        FROM review 
        WHERE restaurant_id = $1 
        ORDER BY created_at DESC 
        LIMIT $2 OFFSET $3`,
		restaurantID, limit, offset)

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	reviews := make([]models.Review, 0, limit)

	for rows.Next() {
		var rev models.Review
		err := rows.Scan(
			&rev.ReviewID, &rev.RestaurantID, &rev.UserID,
			&rev.Comment, &rev.Rating, &rev.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reviews = append(reviews, rev)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return reviews, total, nil
}

func (r *Repository) CreateReview(ctx context.Context, review models.Review) (models.Review, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO review (restaurant_id, user_id, comment, rating) 
         VALUES ($1, $2, $3, $4) 
         RETURNING review_id, created_at`,
		review.RestaurantID, review.UserID, review.Comment, review.Rating,
	).Scan(&review.ReviewID, &review.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code.Name() {
			case "unique_violation":
				return models.Review{}, models.ErrReviewAlreadyExists
			}
		}
		return models.Review{}, err
	}
	return review, nil
}

func (r *Repository) UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error) {
	var updated models.Review
	err := r.db.QueryRowContext(ctx, "UPDATE review "+
		"SET comment = COALESCE($1, comment), "+
		"rating = COALESCE($2, rating) "+
		"WHERE review_id = $3 "+
		"RETURNING review_id, restaurant_id, user_id, comment, rating, created_at",
		review.Comment, review.Rating, review.ReviewID).
		Scan(&updated.ReviewID, &updated.RestaurantID, &updated.UserID, &updated.Comment, &updated.Rating, &updated.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Review{}, models.ErrReviewNotFound
		}
		return models.Review{}, err
	}
	return updated, nil
}

func (r *Repository) DeleteReview(ctx context.Context, reviewID int) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM review WHERE review_id = $1", reviewID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return models.ErrReviewNotFound
	}
	return nil
}

func (r *Repository) GetRestaurantStats(ctx context.Context, restaurantID int) (models.RestaurantStats, error) {
	var stats models.RestaurantStats
	stats.RestaurantID = restaurantID
	err := r.db.QueryRowContext(ctx, "SELECT average_rating, review_count FROM restaurant_stats WHERE restaurant_id = $1", restaurantID).
		Scan(&stats.AverageRating, &stats.ReviewCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.RestaurantStats{RestaurantID: restaurantID}, nil
		}
		return models.RestaurantStats{}, err
	}
	return stats, nil
}
