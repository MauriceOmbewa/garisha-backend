package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig holds the values used to build CORS response headers.
type CORSConfig struct {
	// AllowedOrigins is the list of exact origins that may make cross-origin
	// requests.  Must be explicit origins (not "*") when AllowCredentials
	// is true, because browsers reject wildcard + credentials.
	AllowedOrigins []string

	// AllowedOriginPatterns is a list of domain suffixes that are allowed.
	// Any origin ending with one of these suffixes is permitted.
	// e.g. ".garisha.co.ke" allows automart.garisha.co.ke, xyz.garisha.co.ke etc.
	AllowedOriginPatterns []string

	// AllowedMethods lists the HTTP methods clients are permitted to use.
	AllowedMethods []string

	// AllowedHeaders lists the request headers clients may include.
	AllowedHeaders []string

	// AllowCredentials sets Access-Control-Allow-Credentials: true.
	// Required for HttpOnly cookie-based auth to work cross-origin.
	// When true, AllowedOrigins must not contain "*".
	AllowCredentials bool
}

// DefaultCORSConfig returns a configuration suitable for local development
// that allows cookies (credentials) from the common local frontend origins.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		// Explicit origins required when AllowCredentials is true.
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://localhost:5174",
			"http://localhost:3000",
			"http://localhost:8008",
		},
		// Any subdomain of garisha.co.ke is allowed (tenant portals).
		AllowedOriginPatterns: []string{
			".garisha.co.ke",
		},
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
			"X-Tenant-ID",
			"X-Tenant-Slug",
			"X-Branch-ID",
		},
		AllowCredentials: true, // required for HttpOnly cookie auth
	}
}

// CORS handles Cross-Origin Resource Sharing headers and responds to
// pre-flight OPTIONS requests without forwarding them to the next handler.
//
// When AllowCredentials is true the response mirrors the request's Origin
// header (if it is in the allowed list) rather than using a wildcard —
// browsers require an exact match when credentials are involved.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	// Build a fast lookup set for exact allowed origins.
	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		originSet[o] = struct{}{}
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")

	isAllowed := func(origin string) bool {
		// Exact match
		if _, ok := originSet[origin]; ok {
			return true
		}
		// Pattern match (suffix) — e.g. ".garisha.co.ke" matches
		// "https://automart.garisha.co.ke"
		for _, pattern := range cfg.AllowedOriginPatterns {
			if strings.HasSuffix(origin, pattern) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && isAllowed(origin) {
				// Echo the exact origin back — required when credentials are enabled.
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Vary must include Origin so intermediate caches don't serve
				// one origin's response to another.
				w.Header().Add("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Short-circuit pre-flight requests; no body needed.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
