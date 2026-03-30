package pkg

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

type SponsorshipRequest struct {
	ID   int `json:"id"`
	Tier int `json:"tier" validate:"required,min=1,max=4"`
}

type SponsorshipResponse struct {
	ID    int       `json:"id"`
	Tier  int       `json:"tier" validate:"required,min=1,max=4"`
	Until time.Time `json:"until"`
}

type Sponsorship struct {
	ID         int       `json:"id"`
	Tier       int       `json:"tier" validate:"required,min=1,max=4"`
	Until      time.Time `json:"until"`
	Categories []string  `json:"categories"`
}

type RestaurantDetails struct {
	ID         int      `json:"id"`
	Categories []string `json:"categories"`
	OwnerID    int      `json:"owner_id"`
}

var validate = validator.New()

func ValidateSponsorshipRequest(s SponsorshipRequest) error {
	if err := validate.Struct(s); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
