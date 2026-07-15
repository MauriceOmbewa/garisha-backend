package auth

import (
	"context"
	"fmt"
	"log/slog"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
)

// Service implements the authentication business logic.
type Service struct {
	repo           *Repository
	jwtManager     *platformauth.Manager
	googleVerifier *platformauth.GoogleVerifier
	log            *slog.Logger
}

// NewService constructs a Service with all required dependencies.
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

// LoginWithGoogle verifies a Google ID token, finds or creates the user for
// the given tenant, and returns a JWT token pair.
//
// Flow:
//  1. Verify the Google ID token with Google's public keys.
//  2. Look up the user by (tenantID, googleSub).
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

		user, err = s.repo.Create(ctx, CreateUserParams{
			TenantID:  &tenantID,
			GoogleSub: identity.Sub,
			Email:     identity.Email,
			Name:      identity.Name,
			AvatarURL: avatar,
			Role:      "customer", // default role; admins are promoted separately
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

	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tidStr, user.Role)
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

	tokens, err := s.jwtManager.IssueTokenPair(user.ID, tidStr, user.Role)
	if err != nil {
		return nil, fmt.Errorf("auth: refresh: issue tokens: %w", err)
	}

	return tokens, nil
}

// Me returns the authenticated user by ID extracted from the JWT claims.
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
