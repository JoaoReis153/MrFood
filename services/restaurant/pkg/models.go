package pkg

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Restaurant struct {
	ID            int32    `json:"id"`
	SponsorTier   int32    `json:"sponsor_tier"`
	MaxSlots      int32    `json:"max_slots"`
	OwnerID       int32    `json:"owner_id"`
	ReviewCount   *int32   `json:"review_count,omitempty"`
	Latitude      float64  `json:"latitude"`
	Longitude     float64  `json:"longitude"`
	AverageRating *float64 `json:"average_rating,omitempty"`
	Name          string   `json:"name"`
	OwnerName     string   `json:"owner_name"`
	Address       string   `json:"address"`
	MediaURL      string   `json:"media_url"`
	OpeningTime   string   `json:"opening_time"`
	ClosingTime   string   `json:"closing_time"`
	Categories    []string `json:"categories"`
}

type RestaurantCreateRequest struct {
	OwnerID     int32    `json:"owner_id"`
	MaxSlots    int32    `json:"max_slots"`
	Longitude   float64  `json:"longitude"`
	Latitude    float64  `json:"latitude"`
	Address     string   `json:"address"`
	Name        string   `json:"name"`
	OpeningTime string   `json:"opening_time"`
	ClosingTime string   `json:"closing_time"`
	Categories  []string `json:"categories"`
}

type WorkingHoursResponse struct {
	TimeStart time.Time `json:"time_start"`
	TimeEnd   time.Time `json:"time_end"`
	MaxSlots  int32     `json:"max_slots"`
}

type RestaurantStats struct {
	RestaurantID  int32   `json:"restaurant_id"`
	AverageRating float64 `json:"average_rating"`
	ReviewCount   int32   `json:"review_count"`
}

func (r *Restaurant) ValidateCreateRequest() error {
	if r == nil {
		return errors.New("restaurant payload is required")
	}

	if err := validateCoordinates(r.Latitude, r.Longitude); err != nil {
		return err
	}

	openingTime, err := validateClockField("opening_time", r.OpeningTime)
	if err != nil {
		return err
	}
	closingTime, err := validateClockField("closing_time", r.ClosingTime)
	if err != nil {
		return err
	}

	openingRounded := roundUpToNearestHour(openingTime)
	closingRounded := roundUpToNearestHour(closingTime)

	if !openingRounded.Before(closingRounded) {
		return errors.New("opening_time must be earlier than closing_time")
	}

	r.OpeningTime = openingRounded.Format(time.TimeOnly)
	r.ClosingTime = closingRounded.Format(time.TimeOnly)

	if err := validateUniqueCategories(r.Categories); err != nil {
		return err
	}

	return nil
}

func validateCoordinates(latitude, longitude float64) error {
	if latitude < -90 || latitude > 90 {
		return errors.New("latitude must be between -90 and 90")
	}
	if longitude < -180 || longitude > 180 {
		return errors.New("longitude must be between -180 and 180")
	}
	return nil
}

func validateClockField(fieldName, value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("%s is required", fieldName)
	}
	parsed, err := time.Parse(time.TimeOnly, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use format %q", fieldName, time.TimeOnly)
	}
	return parsed, nil
}

func roundUpToNearestHour(value time.Time) time.Time {
	if value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0 {
		return value
	}
	return value.Truncate(time.Hour).Add(time.Hour)
}

func validateUniqueCategories(categories []string) error {
	seen := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		normalized := strings.ToLower(strings.TrimSpace(category))
		if _, exists := seen[normalized]; exists {
			return errors.New("categories must be unique")
		}
		seen[normalized] = struct{}{}
	}
	return nil
}
