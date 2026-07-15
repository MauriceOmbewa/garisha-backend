package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig holds the values used to build CORS response headers.
type CORSConfig struct {
	// AllowedOrigins is the list of origins that may make cross-origin
	// requests.  Use ["*"] to allow any origin (not recommended in
	// production).
	AllowedOrigins []string

	// AllowedMethods lists the HTTP methods clients are permitted to use.
	AllowedMethods []string

	// AllowedHeaders lists the request headers clients may include.
	AllowedHeaders []string
}

// DefaultCORSConfig returns a permissive configuration suitable for local
// development.  Override with a stricter config in production.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
		},
	}
}

// CORS handles Cross-Origin Resource Sharing headers and responds to
// pre-flight OPTIONS requests without forwarding them to the next handler.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	origins := strings.Join(cfg.AllowedOrigins, ", ")
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)

			// Short-circuit pre-flight requests; no body needed.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
