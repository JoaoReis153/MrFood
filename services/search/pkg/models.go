package pkg

type SearchQuery struct {
	Page   int32
	Limit  int32
	Filter SearchFilters
}

type SearchFilters struct {
	Category   *string
	NameSuffix *string
	FullName   *string
	Location   *LocationRadius
}

type LocationRadius struct {
	Latitude     float64
	Longitude    float64
	RadiusMeters float64
}

type RestaurantSearchResult struct {
	ID         int32    `json:"id"`
	Name       string   `json:"name"`
	Latitude   float64  `json:"latitude"`
	Longitude  float64  `json:"longitude"`
	Address    string   `json:"address"`
	Categories []string `json:"categories"`
	MediaURL   *string  `json:"media_url,omitempty"`
}

type SearchPaginatedResult struct {
	Data       []RestaurantSearchResult
	Pagination Pagination
}

type Pagination struct {
	Page  int32
	Limit int32
	Total int32
	Pages int32
}
