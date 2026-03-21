package repository

import (
	models "MrFood/services/review/pkg"
	sql "database/sql"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetReviews(restaurantID, page, limit int) ([]models.Review, int, error) {
	offset := (page - 1) * limit
	rows, err := r.db.Query(
		"SELECT review_id, restaurant_id, user_id, comment, rating, created_at, COUNT(*) OVER() as total "+
			"FROM review "+
			"WHERE restaurant_id = $1 "+
			"ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		restaurantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reviews []models.Review
	var total int
	for rows.Next() {
		var review models.Review
		err := rows.Scan(&review.ReviewID, &review.RestaurantID, &review.UserID, &review.Comment, &review.Rating, &review.CreatedAt, &total)
		if err != nil {
			return nil, 0, err
		}
		reviews = append(reviews, review)
	}
	return reviews, total, nil
}

func (r *Repository) CreateReview(review models.Review) (int, error) {
	var reviewID int
	err := r.db.QueryRow("INSERT INTO review (restaurant_id, user_id, comment, rating) "+
		"VALUES ($1, $2, $3, $4) RETURNING review_id",
		review.RestaurantID, review.UserID, review.Comment, review.Rating).Scan(&reviewID)
	if err != nil {
		return 0, err
	}
	return reviewID, nil
}

func (r *Repository) UpdateReview(review models.Review) error {
	_, err := r.db.Exec("UPDATE review SET comment = $1, rating = $2 WHERE review_id = $3",
		review.Comment, review.Rating, review.ReviewID)
	return err
}

func (r *Repository) DeleteReview(reviewID int) error {
	_, err := r.db.Exec("DELETE FROM review WHERE review_id = $1", reviewID)
	return err
}
