package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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
//
// It accepts tokens from any of the registered client IDs (web, Android, iOS)
// so a single backend endpoint handles all platforms.
type GoogleVerifier struct {
	clientIDs []string // all trusted audiences: web, android, ios
}

// NewGoogleVerifier creates a GoogleVerifier that accepts id_tokens issued
// for any of the provided clientIDs.  Pass at least the web client ID;
// add the Android and iOS client IDs when those apps are ready.
func NewGoogleVerifier(clientIDs ...string) *GoogleVerifier {
	return &GoogleVerifier{clientIDs: clientIDs}
}

// Verify validates idToken against Google's public keys and returns the
// extracted identity.  It tries each registered client ID in turn and
// succeeds as soon as one matches — this lets a single endpoint serve
// tokens issued by the web, Android, or iOS OAuth clients.
func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (*GoogleIdentity, error) {
	var lastErr error
	for _, cid := range v.clientIDs {
		payload, err := idtoken.Validate(ctx, idToken, cid)
		if err != nil {
			lastErr = err
			continue
		}

		identity := &GoogleIdentity{Sub: payload.Subject}
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

	return nil, fmt.Errorf("google: verify id token: %w", lastErr)
}

// ─── Server-side OAuth2 redirect flow ────────────────────────────────────────

// GoogleOAuthProvider manages the server-side OAuth2 authorization-code flow.
// Use this when you want the backend to own the Google redirect rather than
// the frontend JS SDK.
type GoogleOAuthProvider struct {
	oauthCfg       *oauth2.Config
	allowedOrigins map[string]struct{}
}

// NewGoogleOAuthProvider creates a GoogleOAuthProvider.
//
//   - clientID / clientSecret: from Google Cloud Console credentials
//   - redirectURL:             the exact backend callback URL registered in GCP
//                              (e.g. https://api.example.com/api/v1/auth/google/callback)
//   - allowedOrigins:          whitelist of frontend origins that may initiate
//                              the flow (e.g. ["https://www.example.com",
//                              "http://localhost:8008"])
func NewGoogleOAuthProvider(clientID, clientSecret, redirectURL string, allowedOrigins []string) *GoogleOAuthProvider {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = struct{}{}
	}

	return &GoogleOAuthProvider{
		oauthCfg:       cfg,
		allowedOrigins: origins,
	}
}

// IsOriginAllowed returns true if the given origin is in the whitelist.
func (p *GoogleOAuthProvider) IsOriginAllowed(origin string) bool {
	_, ok := p.allowedOrigins[origin]
	return ok
}

// AuthCodeURL builds the Google consent-screen URL.
//
// The state token encodes both a random nonce (for CSRF protection) and the
// caller's origin so the callback can redirect back to the right frontend.
// Format: <random-nonce>|<origin>
// The whole thing is base64url-encoded so it survives the round trip as a
// URL query parameter.
func (p *GoogleOAuthProvider) AuthCodeURL(origin string) (authURL string, err error) {
	nonce, err := randomNonce(16)
	if err != nil {
		return "", fmt.Errorf("google oauth: generate nonce: %w", err)
	}

	raw := nonce + "|" + origin
	state := base64.RawURLEncoding.EncodeToString([]byte(raw))

	return p.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// ExchangeCode exchanges an authorization code for a GoogleIdentity.
// It also validates the state token and returns the originating frontend URL.
func (p *GoogleOAuthProvider) ExchangeCode(ctx context.Context, code, state string) (*GoogleIdentity, string, error) {
	// Decode and parse the state to recover the origin.
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return nil, "", fmt.Errorf("google oauth: decode state: %w", err)
	}

	// state format: <nonce>|<origin>
	// Split on the FIRST "|" only — origins may theoretically contain pipes.
	idx := indexOf(string(raw), '|')
	if idx < 0 {
		return nil, "", fmt.Errorf("google oauth: malformed state")
	}
	origin := string(raw[idx+1:])

	// Validate origin is still in the whitelist (protects against crafted states).
	if !p.IsOriginAllowed(origin) {
		return nil, "", fmt.Errorf("google oauth: origin not allowed: %s", origin)
	}

	// Exchange the code for tokens.
	token, err := p.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("google oauth: exchange code: %w", err)
	}

	// The id_token is bundled inside the OAuth2 token extras.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, "", fmt.Errorf("google oauth: id_token missing from token response")
	}

	// Verify and decode the id_token.
	payload, err := idtoken.Validate(ctx, rawIDToken, p.oauthCfg.ClientID)
	if err != nil {
		return nil, "", fmt.Errorf("google oauth: validate id_token: %w", err)
	}

	identity := &GoogleIdentity{Sub: payload.Subject}
	if v, ok := payload.Claims["email"].(string); ok {
		identity.Email = v
	}
	if v, ok := payload.Claims["email_verified"].(bool); ok {
		identity.EmailVerified = v
	}
	if v, ok := payload.Claims["name"].(string); ok {
		identity.Name = v
	}
	if v, ok := payload.Claims["picture"].(string); ok {
		identity.Picture = v
	}

	return identity, origin, nil
}

// BuildFrontendRedirectURL constructs the URL the user is sent to after a
// successful login. Only the short-lived access token goes in the URL —
// the refresh token is delivered as an HttpOnly cookie by the backend
// before this redirect, so it never needs to appear in the URL.
//
// Result: <origin>/my-yards?access_token=<at>
func BuildFrontendRedirectURL(origin, accessToken string) string {
	u, err := url.Parse(origin + "/my-yards")
	if err != nil {
		return origin + "/my-yards"
	}
	q := u.Query()
	q.Set("access_token", accessToken)
	u.RawQuery = q.Encode()
	return u.String()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func randomNonce(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
