package pkg

type Receipt struct {
	ID                 int32   `json:"id"`
	UserID             int32   `json:"user_id"`
	Ammount            float32 `json:"ammount"`
	PaymentDescription string  `json:"payment_description"`
}

type PaymentRequest struct {
	UserID             int32   `json:"user_id"`
	Ammount            float32 `json:"ammount"`
	PaymentDescription string  `json:"payment_description"`
}
