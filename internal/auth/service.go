package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
)

// ── One-time session code store ───────────────────────────────────────────────
// After the OAuth callback we cannot reliably set cookies on a redirect response
// (browsers block cross-site Set-Cookie during redirect chains).
// Instead we store the token pair server-side under an opaque code for 2 minutes,
// redirect to the frontend with just the code, and the frontend POSTs the code
// to /api/v1/auth/exchange — a normal CORS response where Set-Cookie works.

type pendingSession struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

type sessionStore struct {
	mu   sync.Mutex
	data map[string]pendingSession
}

func newSessionStore() *sessionStore {
	s := &sessionStore{data: make(map[string]pendingSession)}
	go s.gcLoop()
	return s
}

func (s *sessionStore) put(accessToken, refreshToken string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	code := base64.RawURLEncoding.EncodeToString(b)

	s.mu.Lock()
	s.data[code] = pendingSession{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiresAt:    time.Now().Add(2 * time.Minute),
	}
	s.mu.Unlock()
	return code
}

func (s *sessionStore) take(code string) (pendingSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.data[code]
	if !ok || time.Now().After(ps.expiresAt) {
		delete(s.data, code)
		return pendingSession{}, false
	}
	delete(s.data, code) // one-time use
	return ps, true
}

func (s *sessionStore) gcLoop() {
	for range time.Tick(5 * time.Minute) {
		s.mu.Lock()
		for code, ps := range s.data {
			if time.Now().After(ps.expiresAt) {
				delete(s.data, code)
			}
		}
		s.mu.Unlock()
	}
}

// package-level store shared by Service and Handler
var sessions = newSessionStore()

// Service implements the authentication business logic.
type Service struct {
	repo           *Repository
	jwtManager     *platformauth.Manager
	googleVerifier *platformauth.GoogleVerifier
	googleOAuth    *platformauth.GoogleOAuthProvider
	yardCreator    YardCreator // injected from tenants.Repository; avoids import cycle
	log            *slog.Logger
}

// YardCreator is the minimal interface the auth service needs from the
// tenants domain to create a tenant on behalf of a user.
// tenants.Repository satisfies this interface.
type YardCreator interface {
	CreateTenant(ctx context.Context, p TenantCreateInput) (tenantID string, err error)
}

// TenantCreateInput is passed to YardCreator.CreateTenant.
type TenantCreateInput struct {
	Name  string
	Slug  string
	Email string
	Phone *string
	Plan  string
}

// YardCreatorFunc is a function adapter that satisfies the YardCreator interface.
// Use it in main.go to wire tenantsRepo.Create without an import cycle.
type YardCreatorFunc func(ctx context.Context, p TenantCreateInput) (string, error)

func (f YardCreatorFunc) CreateTenant(ctx context.Context, p TenantCreateInput) (string, error) {
	return f(ctx, p)
}
func NewService(
	repo *Repository,
	jwtManager *platformauth.Manager,
	googleVerifier *platformauth.GoogleVerifier,
	log *slog.Logger,
) *Service {
	return &Service{
		repo:           repo,
		jwtManager:     jwtManager,
		googleVerifier: googleVerifier,
		log:            log,
	}
}

// WithGoogleOAuth attaches the server-side OAuth2 provider.
func (s *Service) WithGoogleOAuth(p *platformauth.GoogleOAuthProvider) {
	s.googleOAuth = p
}

// WithYardCreator injects the tenant creator used by CreateYard.
// Call this in main.go after both repos are constructed.
func (s *Service) WithYardCreator(yc YardCreator) {
	s.yardCreator = yc
}

// LoginWithGoogle verifies a Google ID token, finds or creates the user,
// and returns a JWT token pair.
//
// tenantID is optional. Pass empty string for consumer/public logins where
// users don't belong to a specific tenant yet.
//
// Flow:
//  1. Verify the Google ID token with Google's public keys.
//  2. Look up the user by google_sub (scoped to tenant if provided).
//  3. If not found, create the user automatically (first-time login).
//  4. Reject inactive users.
//  5. Issue and return access + refresh tokens.
func (s *Service) LoginWithGoogle(ctx context.Context, tenantID, idToken string) (*platformauth.TokenPair, *User, error) {
	// 1. Verify with Google.
	identity, err := s.googleVerifier.Verify(ctx, idToken)
	if err != nil {
		s.log.Debug("google id token verification failed", "error", err)
		return nil, nil, apperr.Unauthorized("invalid Google ID token")
	}

	if !identity.EmailVerified {
		return nil, nil, apperr.Unauthorized("Google account email is not verified")
	}

	// 2. Find existing user.
	user, err := s.repo.FindByGoogleSub(ctx, tenantID, identity.Sub)
	if err != nil {
		return nil, nil, apperr.Internal("failed to look up user", err)
	}

	// 3. First-time login — auto-provision the user.
	if user == nil {
		s.log.Info("auto-provisioning new user",
			"email",     identity.Email,
			"tenant_id", tenantID,
		)

		var avatar *string
		if identity.Picture != "" {
			avatar = &identity.Picture
		}

		// Only set TenantID if one was provided
		var tid *string
		if tenantID != "" {
			tid = &tenantID
		}

		user, err = s.repo.Create(ctx, CreateUserParams{
			TenantID:  tid,
			GoogleSub: identity.Sub,
			Email:     identity.Email,
			Name:      identity.Name,
			AvatarURL: avatar,
			Role:      "customer",
		})
		if err != nil {
			return nil, nil, apperr.Internal("failed to create user", err)
		}
	}

	// 4. Reject suspended accounts.
	if !user.IsActive {
		return nil, nil, apperr.Forbidden("your account has been suspended")
	}

	// 5. Issue tokens.
	tidStr := ""
	if user.TenantID != nil {
		tidStr = *user.TenantID
	}

	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tidStr, branchIDStr(user.BranchID), user.Role)
	if err != nil {
		return nil, nil, apperr.Internal("failed to issue tokens", err)
	}

	s.log.Info("user logged in",
		"user_id",   user.ID,
		"tenant_id", tidStr,
		"role",      user.Role,
	)

	return tokens, user, nil
}

// RefreshTokens verifies a refresh token and issues a new token pair.
// The old refresh token is not stored or invalidated at this phase — that
// requires Redis-backed refresh token rotation which is implemented in the
// cache phase.
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*platformauth.TokenPair, error) {
	claims, err := s.jwtManager.Verify(refreshToken, platformauth.TokenTypeRefresh)
	if err != nil {
		return nil, apperr.Unauthorized("invalid or expired refresh token")
	}

	// Re-fetch the user to pick up any role or active-status changes since
	// the token was originally issued.
	user, err := s.repo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, apperr.Internal("failed to look up user", err)
	}

	if user == nil {
		return nil, apperr.Unauthorized("user no longer exists")
	}

	if !user.IsActive {
		return nil, apperr.Forbidden("your account has been suspended")
	}

	tidStr := ""
	if user.TenantID != nil {
		tidStr = *user.TenantID
	}

	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tidStr, branchIDStr(user.BranchID), user.Role)
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: issue tokens: %w", err)
	}

	return tokens, nil
}

// CreateYard is the self-service yard registration flow.
// Any authenticated user who doesn't yet belong to a tenant can call this.
//
// Steps:
//  1. Reject if the user already has a tenant.
//  2. Create a new tenant record via YardCreator.
//  3. Assign the user to that tenant with role "owner".
//  4. Issue new tokens that include the tenant_id in their claims.
func (s *Service) CreateYard(ctx context.Context, userID string, p CreateYardParams) (*platformauth.TokenPair, *User, error) {
	if s.yardCreator == nil {
		return nil, nil, apperr.Internal("yard creator not configured", nil)
	}

	// 1. Ensure user exists and doesn't already have a tenant.
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, nil, apperr.Internal("failed to look up user", err)
	}
	if user == nil {
		return nil, nil, apperr.NotFound("user")
	}
	if user.TenantID != nil {
		return nil, nil, apperr.Conflict("you already belong to a business — leave it before creating a new one")
	}

	// 2. Create the tenant.
	tenantID, err := s.yardCreator.CreateTenant(ctx, TenantCreateInput{
		Name:  p.Name,
		Slug:  p.Slug,
		Email: p.Email,
		Phone: p.Phone,
		Plan:  "trial",
	})
	if err != nil {
		return nil, nil, err // already wrapped by tenants.Repository
	}

	// 3. Assign user to tenant as owner.
	if err := s.repo.AssignTenant(ctx, userID, tenantID, "owner"); err != nil {
		return nil, nil, apperr.Internal("failed to assign user to yard", err)
	}

	// 4. Re-fetch user to get updated tenant_id and role.
	user, err = s.repo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, nil, apperr.Internal("failed to reload user after yard creation", err)
	}

	// 5. Issue fresh tokens with the new tenant_id.
	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tenantID, "", user.Role)
	if err != nil {
		return nil, nil, apperr.Internal("failed to issue tokens", err)
	}

	s.log.Info("yard created", "tenant_id", tenantID, "user_id", userID, "slug", p.Slug)
	return tokens, user, nil
}

// CreateYardParams holds what a user provides when creating a new yard.
type CreateYardParams struct {
	Name         string
	Slug         string
	Email        string
	Phone        *string
	BusinessType *string // optional
}

// branchIDStr converts a *string branch_id to a plain string for JWT embedding.
func branchIDStr(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

func (s *Service) Me(ctx context.Context, userID string) (*User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, apperr.Internal("failed to look up user", err)
	}

	if user == nil {
		return nil, apperr.NotFound("user")
	}

	return user, nil
}

// GoogleOAuthInitiate validates the requesting origin and returns the Google
// consent-screen URL. Returns an error if the origin is not in the allowlist.
func (s *Service) GoogleOAuthInitiate(origin string) (string, error) {
	if s.googleOAuth == nil {
		return "", apperr.Internal("Google OAuth provider not configured", nil)
	}

	if !s.googleOAuth.IsOriginAllowed(origin) {
		return "", apperr.Forbidden("origin not allowed: " + origin)
	}

	authURL, err := s.googleOAuth.AuthCodeURL(origin)
	if err != nil {
		return "", apperr.Internal("failed to build Google auth URL", err)
	}

	return authURL, nil
}

// GoogleOAuthCallback handles the authorization-code callback from Google.
// It exchanges the code, finds-or-creates the user, and returns a token pair
// plus the frontend origin to redirect to.
func (s *Service) GoogleOAuthCallback(ctx context.Context, tenantID, code, state string) (*platformauth.TokenPair, string, error) {
	if s.googleOAuth == nil {
		return nil, "", apperr.Internal("Google OAuth provider not configured", nil)
	}

	identity, origin, err := s.googleOAuth.ExchangeCode(ctx, code, state)
	if err != nil {
		s.log.Debug("google oauth callback exchange failed", "error", err)
		return nil, "", apperr.Unauthorized("google OAuth exchange failed")
	}

	if !identity.EmailVerified {
		return nil, "", apperr.Unauthorized("Google account email is not verified")
	}

	// Find or create the user. tenantID may be empty for consumer logins.
	user, err := s.repo.FindByGoogleSub(ctx, tenantID, identity.Sub)
	if err != nil {
		return nil, "", apperr.Internal("failed to look up user", err)
	}

	if user == nil {
		s.log.Info("auto-provisioning new user via OAuth redirect",
			"email",     identity.Email,
			"tenant_id", tenantID,
		)

		var avatar *string
		if identity.Picture != "" {
			avatar = &identity.Picture
		}

		var tid *string
		if tenantID != "" {
			tid = &tenantID
		}

		user, err = s.repo.Create(ctx, CreateUserParams{
			TenantID:  tid,
			GoogleSub: identity.Sub,
			Email:     identity.Email,
			Name:      identity.Name,
			AvatarURL: avatar,
			Role:      "customer",
		})
		if err != nil {
			return nil, "", apperr.Internal("failed to create user", err)
		}
	}

	if !user.IsActive {
		return nil, "", apperr.Forbidden("your account has been suspended")
	}

	tidStr := ""
	if user.TenantID != nil {
		tidStr = *user.TenantID
	}

	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tidStr, branchIDStr(user.BranchID), user.Role)
	if err != nil {
		return nil, "", apperr.Internal("failed to issue tokens", err)
	}

	s.log.Info("user logged in via OAuth redirect",
		"user_id",   user.ID,
		"tenant_id", tidStr,
		"origin",    origin,
	)

	return tokens, origin, nil
}
