// Package errors defines the application's error taxonomy.
//
// All business logic and handlers return typed errors from this package.
// The Handle function translates them to the correct HTTP status code and
// JSON response, keeping that mapping in exactly one place.
//
// Usage in a handler:
//
//	vehicle, err := svc.GetVehicle(ctx, id)
//	if err != nil {
//	    apperr.Handle(w, r, err, log)
//	    return
//	}
package errors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// ─── Error kinds ─────────────────────────────────────────────────────────────

// Kind classifies an application error so Handle can map it to an HTTP status.
type Kind uint8

const (
	KindNotFound     Kind = iota // 404 — resource does not exist
	KindUnauthorized             // 401 — not authenticated
	KindForbidden                // 403 — authenticated but not authorised
	KindConflict                 // 409 — state conflict (duplicate, etc.)
	KindValidation               // 422 — request failed input validation
	KindBadRequest               // 400 — malformed request (bad JSON, etc.)
	KindInternal                 // 500 — unexpected server error
)

// ─── AppError ────────────────────────────────────────────────────────────────

// AppError is the standard error type returned by services and handlers.
// It carries a Kind for HTTP mapping, a human-readable message for the
// client, and an optional underlying cause for server-side logging.
type AppError struct {
	Kind    Kind
	Message string // safe to surface to the client
	Cause   error  // internal detail, never sent to the client
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap allows errors.Is / errors.As to traverse the chain.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// ─── Constructors ─────────────────────────────────────────────────────────────

// New creates an AppError with the given kind and message.
func New(kind Kind, message string) *AppError {
	return &AppError{Kind: kind, Message: message}
}

// Wrap creates an AppError that wraps an underlying cause.
func Wrap(kind Kind, message string, cause error) *AppError {
	return &AppError{Kind: kind, Message: message, Cause: cause}
}

// NotFound returns a KindNotFound error.
func NotFound(resource string) *AppError {
	return New(KindNotFound, fmt.Sprintf("%s not found", resource))
}

// Unauthorized returns a KindUnauthorized error.
func Unauthorized(message string) *AppError {
	return New(KindUnauthorized, message)
}

// Forbidden returns a KindForbidden error.
func Forbidden(message string) *AppError {
	return New(KindForbidden, message)
}

// Conflict returns a KindConflict error.
func Conflict(message string) *AppError {
	return New(KindConflict, message)
}

// BadRequest returns a KindBadRequest error.
func BadRequest(message string) *AppError {
	return New(KindBadRequest, message)
}

// Internal returns a KindInternal error wrapping a cause.
func Internal(message string, cause error) *AppError {
	return Wrap(KindInternal, message, cause)
}

// ─── HTTP mapping ─────────────────────────────────────────────────────────────

// httpStatus maps an error Kind to its canonical HTTP status code.
func httpStatus(k Kind) int {
	switch k {
	case KindNotFound:
		return http.StatusNotFound
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindConflict:
		return http.StatusConflict
	case KindValidation:
		return http.StatusUnprocessableEntity
	case KindBadRequest:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// ─── RequestIDKey ─────────────────────────────────────────────────────────────

// RequestIDFromContext is a function variable that Handle uses to extract the
// request ID for log annotations.  It is set by the middleware package at
// init time to avoid an import cycle between errors ↔ middleware.
// Defaults to a no-op so Handle is safe to call without middleware in tests.
var RequestIDFromContext func(ctx context.Context) string = func(ctx context.Context) string {
	return ""
}

// ─── Handle ───────────────────────────────────────────────────────────────────

// Handle translates err into the appropriate HTTP response and writes it.
// It is the single exit point for all error handling in handlers.
//
// Rules:
//   - *AppError         → mapped status + client message
//   - *ValidationErrors → 422 + field error list
//   - anything else     → 500 + generic message (cause logged, not exposed)
func Handle(w http.ResponseWriter, r *http.Request, err error, log *slog.Logger) {
	reqID := RequestIDFromContext(r.Context())

	var appErr *AppError
	if errors.As(err, &appErr) {
		if appErr.Kind == KindInternal {
			log.Error("internal error",
				"error",      appErr.Cause,
				"message",    appErr.Message,
				"request_id", reqID,
			)
		} else {
			log.Debug("application error",
				"kind",       appErr.Kind,
				"message",    appErr.Message,
				"request_id", reqID,
			)
		}

		response.Error(w, httpStatus(appErr.Kind), appErr.Message, nil, log)
		return
	}

	var valErrs *ValidationErrors
	if errors.As(err, &valErrs) {
		log.Debug("validation error",
			"fields",     valErrs.Fields,
			"request_id", reqID,
		)

		response.Error(w, http.StatusUnprocessableEntity, "validation failed", valErrs.Fields, log)
		return
	}

	// Unknown error — do not leak internals.
	log.Error("unexpected error",
		"error",      err,
		"request_id", reqID,
	)

	response.Error(w, http.StatusInternalServerError, "an unexpected error occurred", nil, log)
}
