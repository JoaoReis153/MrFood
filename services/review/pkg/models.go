package pkg

import (
	"errors"
	"time"
)

var (
	ErrInvalidRating       = errors.New("Rating must be between 1 and 5")
	ErrInvalidComment      = errors.New("Comment is required and must be less than 100 characters")
	ErrInvalidRestaurantID = errors.New("Restaurant ID must be a positive integer")
	ErrInvalidUserID       = errors.New("User ID must be a positive integer")
	ErrInvalidReviewID     = errors.New("Review ID must be a positive integer")
	ErrReviewAlreadyExists = errors.New("User has already reviewed this restaurant")
	ErrReviewNotFound      = errors.New("Review not found")
	ErrRestaurantNotFound  = errors.New("Restaurant not found")
	ErrLimitTooLarge       = errors.New("Limit must be less than or equal to 100")
)

type Review struct {
	ReviewID     int       `json:"id"`
	RestaurantID int       `json:"restaurant_id"`
	UserID       int       `json:"user_id"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type UpdateReview struct {
	ReviewID int     `json:"id"`
	Rating   *int    `json:"rating,omitempty"`
	Comment  *string `json:"comment,omitempty"`
}

type RestaurantStats struct {
	RestaurantID  int     `json:"restaurant_id"`
	AverageRating float64 `json:"average_rating"`
	ReviewCount   int     `json:"review_count"`
}

type ReviewsPage struct {
	Reviews    []Review   `json:"reviews"`
	Pagination Pagination `json:"pagination"`
}
