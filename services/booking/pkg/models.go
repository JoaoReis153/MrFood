package pkg

import "time"

type Booking struct {
	ID           int32     `json:"id"`
	UserID       int64     `json:"user_id"`
	RestaurantID int64     `json:"restaurant_id"`
	PeopleCount  int32     `json:"people_count"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
}

type DeleteBooking struct {
	BookingID int32 `json:"booking_id"`
	UserID    int64 `json:"user_id"`
}

type RestaurantSlots struct {
	ID           int32     `json:"id"`
	RestaurantID int64     `json:"restaurant_id"`
	WeekDay      int32     `json:"week_day"`
	MaxSlots     int32     `json:"max_slots"`
	CurrentSlots int32     `json:"current_slots"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
}

type WorkingHours struct {
	RestaurantID int64     `json:"restaurant_id"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
}
