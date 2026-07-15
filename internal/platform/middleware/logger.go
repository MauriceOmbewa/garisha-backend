package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code written
// by downstream handlers.  The standard library's ResponseWriter does not
// expose the written status after the fact.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Logger logs a structured line for every HTTP request that includes the
// method, path, status code, latency, and request ID.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK, // default if WriteHeader is never called
			}

			next.ServeHTTP(rw, r)

			log.Info("request",
				"method",     r.Method,
				"path",       r.URL.Path,
				"status",     rw.status,
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", GetRequestID(r.Context()),
				"remote_ip",  r.RemoteAddr,
			)
		})
	}
}
