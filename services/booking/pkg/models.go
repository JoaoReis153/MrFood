package pkg

import "time"

type CreateBooking struct {
	ID           int32     `json:"id"`
	UserID       int32     `json:"user_id"`
	RestaurantID int32     `json:"restaurant_id"`
	PeopleCount  int32     `json:"people_count"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
	MaxSlots     int32     `json:"max_slots"`
}

type DeleteBooking struct {
	BookingID int32 `json:"booking_id"`
	UserID    int32 `json:"user_id"`
}

type WorkingHours struct {
	RestaurantID int32     `json:"restaurant_id"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
	MaxSlots     int32     `json:"max_slots"`
}
