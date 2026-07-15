package audit

import (
	"context"
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for the audit log.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Query methods (admin only) ────────────────────────────────────────────────

// List returns audit log entries for the tenant, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Log, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	entries, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list audit logs", err)
	}

	return entries, nil
}

// GetByID returns a single audit log entry scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Log, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	entry, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get audit log entry", err)
	}

	if entry == nil || entry.TenantID != tenantID {
		return nil, apperr.NotFound("audit log entry")
	}

	return entry, nil
}

// ── Write methods (internal — called by other domain services) ────────────────

// RecordInput carries all fields for recording an audit event.
type RecordInput struct {
	TenantID     string
	Action       string
	ResourceType string
	ResourceID   *string
	Changes      map[string]any
	Status       LogStatus     // defaults to StatusSuccess
	ErrorMessage *string
	// Request context fields — populated from http.Request when available.
	ActorID   *string
	ActorEmail *string
	ActorRole  *string
	IPAddress  *string
	UserAgent  *string
	RequestID  *string
}

// Record appends a single audit entry.  Errors are logged but never propagated
// back to the caller — an audit failure must never break the primary operation.
func (s *Service) Record(ctx context.Context, in RecordInput) {
	if in.Status == "" {
		in.Status = StatusSuccess
	}

	if in.Changes == nil {
		in.Changes = map[string]any{}
	}

	if _, err := s.repo.Append(ctx, AppendParams{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorEmail:   in.ActorEmail,
		ActorRole:    in.ActorRole,
		Action:       in.Action,
		ResourceType: in.ResourceType,
		ResourceID:   in.ResourceID,
		Changes:      in.Changes,
		IPAddress:    in.IPAddress,
		UserAgent:    in.UserAgent,
		RequestID:    in.RequestID,
		Status:       in.Status,
		ErrorMessage: in.ErrorMessage,
	}); err != nil {
		s.log.Error("audit: failed to append log entry",
			"action",        in.Action,
			"resource_type", in.ResourceType,
			"error",         err,
		)
	}
}

// ── Logger helper — convenience wrapper for HTTP handlers ─────────────────────

// Logger is a thin helper that extracts actor and request context from an
// *http.Request and calls Record.  Other domain handlers embed this or
// receive it via dependency injection.
type Logger struct {
	svc *Service
}

// NewLogger creates a Logger backed by the given Service.
func NewLogger(svc *Service) *Logger {
	return &Logger{svc: svc}
}

// LogEvent records an audit event from an HTTP handler context.
// It extracts actor identity from JWT claims and request metadata automatically.
func (l *Logger) LogEvent(r *http.Request, tenantID, action, resourceType string, resourceID *string, changes map[string]any) {
	in := RecordInput{
		TenantID:     tenantID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
		Status:       StatusSuccess,
	}

	// Extract actor from JWT claims if present.
	if claims := middleware.GetClaims(r.Context()); claims != nil {
		in.ActorID = &claims.UserID
		in.ActorRole = &claims.Role
	}

	// Request metadata.
	ip := extractIP(r)
	if ip != "" {
		in.IPAddress = &ip
	}

	ua := r.Header.Get("User-Agent")
	if ua != "" {
		in.UserAgent = &ua
	}

	reqID := middleware.GetRequestID(r.Context())
	if reqID != "" {
		in.RequestID = &reqID
	}

	l.svc.Record(r.Context(), in)
}

// LogFailure records a failed audit event (e.g. unauthorised access attempt).
func (l *Logger) LogFailure(r *http.Request, tenantID, action, resourceType string, errMsg string) {
	msg := errMsg
	in := RecordInput{
		TenantID:     tenantID,
		Action:       action,
		ResourceType: resourceType,
		Status:       StatusFailure,
		ErrorMessage: &msg,
	}

	if claims := middleware.GetClaims(r.Context()); claims != nil {
		in.ActorID = &claims.UserID
		in.ActorRole = &claims.Role
	}

	ip := extractIP(r)
	if ip != "" {
		in.IPAddress = &ip
	}

	reqID := middleware.GetRequestID(r.Context())
	if reqID != "" {
		in.RequestID = &reqID
	}

	l.svc.Record(r.Context(), in)
}

// extractIP returns the best-guess client IP from the request.
func extractIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// Take only the first address (leftmost = original client).
		for i, c := range v {
			if c == ',' {
				return v[:i]
			}
		}
		return v
	}

	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}

	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}

	return addr
}
