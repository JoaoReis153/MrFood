package pkg

type User struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Restaurant struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	Tags      string   `json:"tags"`
	Price     string   `json:"price"`
	Phone     string   `json:"phone"`
	Rating    float32  `json:"rating"`
	Sponsored bool     `json:"sponsored"`
	Location  Location `json:"location"`
}

type Review struct {
	Id           string `json:"id"`
	Rating       int    `json:"rating"`
	Comment      string `json:"comment"`
	Categories   string `json:"categories"`
	UserId       string `json:"userid"`
	RestaurantId string `json:"restaurantid"`
	CreatedAt    string `json:"createdAt"`
}

type Reservation struct {
	Id             string `json:"id"`
	UserId         string `json:"userId"`
	RestaurantId   string `json:"restaurantId"`
	DateTime       string `json:"dateTime"`
	NumberOfPeople int    `json:"numberOfPeople"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
