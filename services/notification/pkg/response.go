package pkg

type SuccessResponse[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
