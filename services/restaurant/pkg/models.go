package pkg

type Restaurant struct {
	ID           int32    `json:"id"`
	SponsorTier  int32    `json:"sponsor_tier"`
	MaxSlots     int32    `json:"max_slots"`
	OwnerID      int32    `json:"owner_id"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	Name         string   `json:"name"`
	OwnerName    string   `json:"owner_name"`
	Address      string   `json:"address"`
	MediaURL     string   `json:"media_url"`
	WorkingHours []string `json:"working_hours" validate:"len=7"`
	Categories   []string `json:"categories"`
}

type RestaurantCreateRequest struct {
	OwnerID      int32    `json:"owner_id"`
	MaxSlots     int32    `json:"max_slots"`
	Longitude    float64  `json:"longitude"`
	Latitude     float64  `json:"latitude"`
	Address      string   `json:"address"`
	Name         string   `json:"name"`
	WorkingHours []string `json:"working_hours" validate:"len=7"`
	Categories   []string `json:"categories"`
}
