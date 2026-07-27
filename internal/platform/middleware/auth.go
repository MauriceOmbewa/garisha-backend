package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// claimsKey is the context key under which verified JWT claims are stored.
type claimsKey struct{}

// Authenticate extracts the Bearer token from the Authorization header,
// verifies it as an access token, and stores the claims in the request
// context.  Returns 401 if the token is absent or invalid.
func Authenticate(jwtManager *platformauth.Manager, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, err := bearerToken(r)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "missing or malformed authorization header", nil, log)
				return
			}

			claims, err := jwtManager.Verify(tokenStr, platformauth.TokenTypeAccess)
			if err != nil {
				log.Debug("invalid access token", "error", err, "request_id", GetRequestID(r.Context()))
				response.Error(w, http.StatusUnauthorized, "invalid or expired token", nil, log)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims retrieves the JWT claims stored by Authenticate middleware.
// Returns nil if the context carries no claims (unauthenticated request).
func GetClaims(ctx context.Context) *platformauth.Claims {
	claims, _ := ctx.Value(claimsKey{}).(*platformauth.Claims)
	return claims
}

// RequireClaims is a helper that retrieves claims and writes a 401 if they
// are absent.  Handlers call this instead of GetClaims when authentication
// is mandatory.
func RequireClaims(ctx context.Context, w http.ResponseWriter, log *slog.Logger) (*platformauth.Claims, bool) {
	claims := GetClaims(ctx)
	if claims == nil {
		response.Error(w, http.StatusUnauthorized, apperr.Unauthorized("authentication required").Message, nil, log)
		return nil, false
	}
	return claims, true
}

// bearerToken extracts the access token from the request.
// Priority:
//  1. Authorization: Bearer <token> header  — used by mobile apps and Postman
//  2. garisha_at HttpOnly cookie             — used by web browsers
func bearerToken(r *http.Request) (string, error) {
	// 1. Header (mobile / API clients)
	if header := r.Header.Get("Authorization"); header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return "", apperr.Unauthorized("authorization header must be: Bearer <token>")
		}
		return parts[1], nil
	}

	// 2. HttpOnly cookie (web browsers)
	if cookie, err := r.Cookie("garisha_at"); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	return "", apperr.Unauthorized("authentication required")
}
