package pkg

import "time"

type Receipt struct {
	ID                 int32     `json:"id"`
	UserID             int64     `json:"user_id"`
	UserEmail          string    `json:"user_email"`
	IdempotencyKey     string    `json:"idempotency_key"`
	Amount             float32   `json:"amount"`
	PaymentDescription string    `json:"payment_description"`
	PaymentStatus      string    `json:"payment_status"`
	PaymentType        string    `json:"payment_type"`
	CreatedAt          time.Time `json:"created_at"`
}
