package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// GoogleIdentity holds the verified claims extracted from a Google ID token.
type GoogleIdentity struct {
	Sub           string // Google's unique user identifier — never changes
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// GoogleVerifier verifies Google ID tokens using Google's public keys.
// It wraps the official google.golang.org/api/idtoken package.
type GoogleVerifier struct {
	clientID string
}

// NewGoogleVerifier creates a GoogleVerifier for the given OAuth client ID.
func NewGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{clientID: clientID}
}

// Verify validates idToken against Google's public keys and returns the
// extracted identity.  It confirms the token's audience matches clientID
// so tokens issued for other applications are rejected.
func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (*GoogleIdentity, error) {
	payload, err := idtoken.Validate(ctx, idToken, v.clientID)
	if err != nil {
		return nil, fmt.Errorf("google: verify id token: %w", err)
	}

	identity := &GoogleIdentity{
		Sub: payload.Subject,
	}

	if email, ok := payload.Claims["email"].(string); ok {
		identity.Email = email
	}

	if verified, ok := payload.Claims["email_verified"].(bool); ok {
		identity.EmailVerified = verified
	}

	if name, ok := payload.Claims["name"].(string); ok {
		identity.Name = name
	}

	if picture, ok := payload.Claims["picture"].(string); ok {
		identity.Picture = picture
	}

	return identity, nil
}
