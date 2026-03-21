package pkg

import "time"

type Example struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Review struct {
	ReviewID     int       `json:"id"`
	RestaurantID int       `json:"restaurant_id"`
	UserID       int       `json:"user_id"`
	Comment      string    `json:"comment"`
	Rating       int       `json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
}

type ReviewsPage struct {
	Reviews    []Review   `json:"reviews"`
	Pagination Pagination `json:"pagination"`
}
