package pkg

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

type SponsorshipRequest struct {
	ID   int64 `json:"id"`
	Tier int   `json:"tier" validate:"required,min=1,max=4"`
}

type SponsorshipResponse struct {
	ID    int64     `json:"id"`
	Tier  int       `json:"tier" validate:"required,min=1,max=4"`
	Until time.Time `json:"until"`
}

type Sponsorship struct {
	ID         int64     `json:"id"`
	Tier       int       `json:"tier" validate:"required,min=1,max=4"`
	Until      time.Time `json:"until"`
	Categories []string  `json:"categories"`
}

type RestaurantDetails struct {
	ID         int64    `json:"id"`
	Categories []string `json:"categories"`
	OwnerID    int64    `json:"owner_id"`
}

type PaymentRequest struct {
	UserID             int64   `json:"user_id"`
	UserEmail          string  `json:"user_email"`
	IdempotencyKey     string  `json:"idempotency_key"`
	Amount             float32 `json:"amount"`
	PaymentDescription string  `json:"payment_description"`
	PaymentType        string  `json:"payment_type"`
}

var validate = validator.New()

func ValidateSponsorshipRequest(s SponsorshipRequest) error {
	if err := validate.Struct(s); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
