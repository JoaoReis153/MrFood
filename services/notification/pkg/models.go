package pkg

import "errors"

var (
	ErrInvalidUsername   = errors.New("username must not be empty")
	ErrInvalidEmail      = errors.New("email must not be empty")
	ErrEmptyReceipts     = errors.New("receipts list must not be empty")
	ErrRateLimitExceeded = errors.New("rate limit exceeded for this email")
	ErrRedisIncrFailed   = errors.New("failed to increment rate limit counter in Redis")
	ErrSendEmailFailed   = errors.New("failed to send email")
)

type NotificationResponse struct{}
