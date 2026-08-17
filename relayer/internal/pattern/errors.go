package pattern

import (
	"errors"
)

var (
	ErrPayloadTooLarge = errors.New("payload exceeds maximum size")
	ErrInvalidInput    = errors.New("invalid input data")
	ErrNotFound        = errors.New("resource not found")
	ErrUnauthorized    = errors.New("unauthorized access")
	ErrRateLimited     = errors.New("rate limit exceeded")
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTimeout         = errors.New("operation timed out")
	ErrPoolInactive    = errors.New("pool is inactive")
	ErrInsufficientFee = errors.New("fee below minimum threshold")
)
