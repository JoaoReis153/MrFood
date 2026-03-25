package pkg

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type Sponsorship struct {
	ID   int `json:"id"`
	Tier int `json:"tier" validate:"required,min=1,max=4"`
}

var validate = validator.New()

func ValidateSponsorshipRequest(s Sponsorship) error {
	if err := validate.Struct(s); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
