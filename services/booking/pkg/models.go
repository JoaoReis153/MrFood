package pkg

import "time"

type Booking struct {
	ID           int32     `json:"id"`
	UserID       int32     `json:"user_id"`
	RestaurantID int32     `json:"restaurant_id"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
	PeopleCount  int32     `json:"people_count"`
}

type HourSlots struct {
	MaxSlots     int32
	CurrentSlots int32
}
