package pkg

import "time"

type Receipt struct {
	ID                 int32     `json:"id"`
	IdempotencyKey     string    `json:"idempotency_key"`
	UserID             int32     `json:"user_id"`
	Ammount            float32   `json:"ammount"`
	PaymentDescription string    `json:"payment_description"`
	PaymentStatus      string    `json:"payment_status"`
	CreatedAt          time.Time `json:"created_at"`
}

type PaymentRequest struct {
	IdempotencyKey     string  `json:"idempotency_key"`
	UserID             int32   `json:"user_id"`
	Ammount            float32 `json:"ammount"`
	PaymentDescription string  `json:"payment_description"`
	PaymentStatus      string  `json:"payment_status"`
}
