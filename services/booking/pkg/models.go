package pkg

import "time"

type Booking struct {
	ID           int32     `json:"id"`
	UserID       int32     `json:"user_id"`
	RestaurantID int32     `json:"restaurant_id"`
	PeopleCount  int32     `json:"people_count"`
	TimeStart    time.Time `json:"time_start"`
	TimeEnd      time.Time `json:"time_end"`
}

type HourSlots struct {
	MaxSlots     int32
	CurrentSlots int32
}
