package usecases

import "errors"

// Sentinel errors the handler layer maps to HTTP status codes via errors.Is.
var (
	ErrEmailAlreadyExists       = errors.New("email is already registered")
	ErrPasswordTooLong          = errors.New("password must be at most 72 bytes")
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrAccountNotFound          = errors.New("account not found")
	ErrInsufficientBalance      = errors.New("insufficient balance")
	ErrSameAccountTransfer      = errors.New("cannot transfer to the same account")
	ErrInvalidAmount            = errors.New("amount must be greater than zero")
	ErrIdempotencyKeyRequired   = errors.New("idempotency key is required")
	ErrIdempotencyKeyTooLong    = errors.New("idempotency key must be at most 255 characters")
	ErrIdempotencyKeyConflict   = errors.New("idempotency key was already used with a different request payload")
	ErrIdempotencyKeyInProgress = errors.New("a request with this idempotency key is still being processed")
)
