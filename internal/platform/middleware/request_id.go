package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// contextKey is an unexported type used as a context key to avoid collisions
// with keys from other packages.
type contextKey string

const requestIDKey contextKey = "requestID"

// RequestID injects a unique identifier into every request's context and
// response headers.  Downstream handlers and middleware retrieve it with
// GetRequestID so it can be included in logs and error responses.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateID()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID stored in ctx by RequestID middleware.
// Returns an empty string if no ID is present.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// generateID returns a 16-byte cryptographically random hex string.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
