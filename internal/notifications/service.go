package notifications

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for notification management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// SendInput carries the fields required to create a notification.
// Used by other domain services to dispatch notifications programmatically.
type SendInput struct {
	TenantID     string
	UserID       string
	Type         string
	Title        string
	Body         string
	ResourceType *string
	ResourceID   *string
}

// ── User-facing methods ───────────────────────────────────────────────────────

// List returns notifications for the authenticated user in ctx.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Notification, error) {
	tenantID, userID := mustGetTenantAndUser(ctx)

	ns, err := s.repo.List(ctx, tenantID, userID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list notifications", err)
	}

	return ns, nil
}

// GetUnreadCount returns the number of unread notifications for the caller.
func (s *Service) GetUnreadCount(ctx context.Context) (UnreadCount, error) {
	tenantID, userID := mustGetTenantAndUser(ctx)

	count, err := s.repo.CountUnread(ctx, tenantID, userID)
	if err != nil {
		return UnreadCount{}, apperr.Internal("failed to count unread notifications", err)
	}

	return UnreadCount{Count: count}, nil
}

// MarkRead marks a single notification as read.
// Enforces ownership — the notification must belong to the calling user.
func (s *Service) MarkRead(ctx context.Context, id string) (*Notification, error) {
	tenantID, userID := mustGetTenantAndUser(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get notification", err)
	}

	if existing == nil || existing.TenantID != tenantID || existing.UserID != userID {
		return nil, apperr.NotFound("notification")
	}

	if existing.IsRead {
		return existing, nil // already read — idempotent
	}

	n, err := s.repo.MarkRead(ctx, id, time.Now().UTC())
	if err != nil {
		return nil, apperr.Internal("failed to mark notification as read", err)
	}

	return n, nil
}

// MarkAllRead marks every unread notification for the calling user as read.
// Returns the count of notifications updated.
func (s *Service) MarkAllRead(ctx context.Context) (int64, error) {
	tenantID, userID := mustGetTenantAndUser(ctx)

	count, err := s.repo.MarkAllRead(ctx, tenantID, userID, time.Now().UTC())
	if err != nil {
		return 0, apperr.Internal("failed to mark all notifications as read", err)
	}

	return count, nil
}

// Delete removes a single notification owned by the calling user.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID, userID := mustGetTenantAndUser(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get notification", err)
	}

	if existing == nil || existing.TenantID != tenantID || existing.UserID != userID {
		return apperr.NotFound("notification")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("notification")
		}
		return apperr.Internal("failed to delete notification", err)
	}

	return nil
}

// DeleteRead removes all read notifications for the calling user.
func (s *Service) DeleteRead(ctx context.Context) (int64, error) {
	tenantID, userID := mustGetTenantAndUser(ctx)

	count, err := s.repo.DeleteAllRead(ctx, tenantID, userID)
	if err != nil {
		return 0, apperr.Internal("failed to delete read notifications", err)
	}

	return count, nil
}

// ── System-facing method ──────────────────────────────────────────────────────

// Send creates a notification programmatically from another domain service.
// It does not require a tenant/user context from the HTTP layer — the caller
// supplies TenantID and UserID explicitly via SendInput.
func (s *Service) Send(ctx context.Context, in SendInput) (*Notification, error) {
	if in.TenantID == "" || in.UserID == "" {
		return nil, apperr.BadRequest("tenant_id and user_id are required to send a notification")
	}

	if in.Title == "" || in.Body == "" {
		return nil, apperr.BadRequest("title and body are required")
	}

	n, err := s.repo.Create(ctx, CreateParams{
		TenantID:     in.TenantID,
		UserID:       in.UserID,
		Type:         in.Type,
		Title:        in.Title,
		Body:         in.Body,
		ResourceType: in.ResourceType,
		ResourceID:   in.ResourceID,
	})
	if err != nil {
		return nil, apperr.Internal("failed to send notification", err)
	}

	s.log.Info("notification sent",
		"notification_id", n.ID,
		"user_id",         in.UserID,
		"type",            in.Type,
	)

	return n, nil
}

// ── Context helpers ───────────────────────────────────────────────────────────

// mustGetTenantAndUser extracts tenantID and userID from the request context.
// Panics if either is missing (indicates a middleware misconfiguration).
func mustGetTenantAndUser(ctx context.Context) (tenantID, userID string) {
	tenantID = tenant.MustGetTenantID(ctx)

	claims := middleware.GetClaims(ctx)
	if claims == nil || claims.UserID == "" {
		panic("notifications: JWT claims not found in context — is Authenticate middleware applied?")
	}

	return tenantID, claims.UserID
}
