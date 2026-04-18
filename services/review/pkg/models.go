package pkg

import (
	"errors"
	"time"
)

var (
	ErrInvalidRating                = errors.New("rating must be between 1 and 5")
	ErrInvalidComment               = errors.New("comment is required and must be less than 100 characters")
	ErrInvalidRestaurantID          = errors.New("restaurant ID must be a positive integer")
	ErrInvalidUserID                = errors.New("user ID must be a positive integer")
	ErrInvalidReviewID              = errors.New("review ID must be a positive integer")
	ErrReviewAlreadyExists          = errors.New("user has already reviewed this restaurant")
	ErrReviewNotFound               = errors.New("review not found")
	ErrRestaurantNotFound           = errors.New("restaurant not found")
	ErrLimitTooLarge                = errors.New("limit must be less than or equal to 100")
	ErrRestaurantServiceUnavailable = errors.New("restaurant details service is currently unavailable")
	ErrForbidden                    = errors.New("access forbidden")
	ErrUnauthenticated              = errors.New("invalid token")
)

type Review struct {
	ReviewID     int64     `json:"id"`
	RestaurantID int64     `json:"restaurant_id"`
	UserID       int64     `json:"user_id"`
	Rating       int32     `json:"rating"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateReview struct {
	RestaurantID int64  `json:"restaurant_id"`
	Rating       int32  `json:"rating"`
	Comment      string `json:"comment"`
}

type UpdateReview struct {
	ReviewID int64   `json:"id"`
	UserID   int64   `json:"user_id,omitempty"`
	Rating   *int32  `json:"rating,omitempty"`
	Comment  *string `json:"comment,omitempty"`
}

type DeleteReview struct {
	ReviewID int64 `json:"id"`
	UserID   int64 `json:"user_id"`
}

type RestaurantStats struct {
	RestaurantID  int64   `json:"restaurant_id"`
	AverageRating float64 `json:"average_rating"`
	ReviewCount   int32   `json:"review_count"`
}

type ReviewsPage struct {
	Reviews    []Review   `json:"reviews"`
	Pagination Pagination `json:"pagination"`
}

type Restaurant struct {
	RestaurantID int64 `json:"restaurant_id"`
}
