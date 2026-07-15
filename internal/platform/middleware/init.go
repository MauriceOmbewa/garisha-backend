package middleware

import (
	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
)

func init() {
	// Wire GetRequestID into the errors package so apperr.Handle can annotate
	// log lines with the request ID without creating an import cycle between
	// errors ↔ middleware.
	apperr.RequestIDFromContext = GetRequestID
}
