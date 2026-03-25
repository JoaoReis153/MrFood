package pkg

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type User struct {
	ID       int32  `json:"id"`
	Username string `json:"name"     validate:"required,min=3,max=32"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

var validate = validator.New()

func ValidateUser(u User) error {
	if err := validate.Struct(u); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
