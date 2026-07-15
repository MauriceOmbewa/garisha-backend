package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Recovery catches any panic that occurs in downstream handlers, logs the
// stack trace, and returns a 500 response so a single bad request cannot
// bring down the entire server process.
func Recovery(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"error",      rec,
						"stack",      string(debug.Stack()),
						"method",     r.Method,
						"path",       r.URL.Path,
						"request_id", GetRequestID(r.Context()),
					)

					response.Error(
						w,
						http.StatusInternalServerError,
						"an unexpected error occurred",
						nil,
						log,
					)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
