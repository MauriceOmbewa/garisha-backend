// Package auth provides JWT token issuance and verification for the platform.
// It is a pure infrastructure concern — it knows nothing about users or tenants.
// Business logic lives in the internal/auth domain module.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType distinguishes access tokens from refresh tokens so a refresh
// token can never be used where an access token is expected.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims is the custom JWT payload embedded in every token issued by this
// platform.  It is stored in the request context after successful verification
// so downstream handlers and middleware can read identity without re-parsing.
type Claims struct {
	UserID   string    `json:"uid"`
	TenantID string    `json:"tid"`
	BranchID string    `json:"bid,omitempty"` // empty = cross-branch access
	Role     string    `json:"role"`
	Type     TokenType `json:"type"`
	jwt.RegisteredClaims
}

// TokenPair holds both tokens returned on a successful login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Manager handles token issuance and validation.
// Construct one via NewManager and inject it wherever tokens are needed.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager creates a Manager.  secret must not be empty.
func NewManager(secret string, accessTTL, refreshTTL time.Duration) (*Manager, error) {
	if secret == "" {
		return nil, errors.New("jwt: secret must not be empty")
	}

	return &Manager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

// IssueTokenPair signs and returns a fresh access + refresh token pair.
// branchID is empty for cross-branch roles (owner, admin, accountant).
func (m *Manager) IssueTokenPair(userID, tenantID, branchID, role string) (*TokenPair, error) {
	access, err := m.sign(userID, tenantID, branchID, role, TokenTypeAccess, m.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("jwt: issue access token: %w", err)
	}

	refresh, err := m.sign(userID, tenantID, branchID, role, TokenTypeRefresh, m.refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("jwt: issue refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// Verify parses tokenStr, validates the signature and standard claims, and
// returns the embedded Claims.  It rejects tokens whose Type field does not
// match expectedType so refresh tokens cannot be used as access tokens.
func (m *Manager) Verify(tokenStr string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("jwt: invalid token")
	}

	if claims.Type != expectedType {
		return nil, fmt.Errorf("jwt: expected %s token, got %s", expectedType, claims.Type)
	}

	return claims, nil
}

// sign creates and signs a token with the given parameters.
func (m *Manager) sign(userID, tenantID, branchID, role string, tokenType TokenType, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := &Claims{
		UserID:   userID,
		TenantID: tenantID,
		BranchID: branchID,
		Role:     role,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", err
	}

	return signed, nil
}
