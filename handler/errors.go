package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rozy97/mini-bank/usecases"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// handleError maps a usecase error to an HTTP response. Unrecognized errors
// are logged with their full detail and reported to the client as an opaque
// 500 so internals never leak.
func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecases.ErrEmailAlreadyExists):
		fail(c, http.StatusConflict, "EMAIL_ALREADY_EXISTS", err.Error())
	case errors.Is(err, usecases.ErrInvalidCredentials):
		fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
	case errors.Is(err, usecases.ErrAccountNotFound):
		fail(c, http.StatusNotFound, "ACCOUNT_NOT_FOUND", err.Error())
	case errors.Is(err, usecases.ErrInsufficientBalance):
		fail(c, http.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", err.Error())
	case errors.Is(err, usecases.ErrSameAccountTransfer),
		errors.Is(err, usecases.ErrInvalidAmount),
		errors.Is(err, usecases.ErrIdempotencyKeyRequired):
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	case errors.Is(err, usecases.ErrIdempotencyKeyConflict),
		errors.Is(err, usecases.ErrIdempotencyKeyInProgress):
		fail(c, http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", err.Error())
	default:
		slog.ErrorContext(c.Request.Context(), "unhandled error", "error", err, "path", c.Request.URL.Path)
		if span := trace.SpanFromContext(c.Request.Context()); span.IsRecording() {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
	}
}
