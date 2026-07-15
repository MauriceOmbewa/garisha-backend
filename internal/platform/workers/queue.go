package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Queue is the PostgreSQL-backed job queue.
type Queue struct {
	db *pgxpool.Pool
}

// NewQueue creates a Queue backed by the given pool.
func NewQueue(db *pgxpool.Pool) *Queue {
	return &Queue{db: db}
}

// ── Column list ───────────────────────────────────────────────────────────────

const jobCols = `
	id, tenant_id,
	type, payload,
	run_at, status, attempts, max_attempts, last_error,
	started_at, completed_at,
	idempotency_key, created_by,
	created_at, updated_at`

// ── Enqueue ───────────────────────────────────────────────────────────────────

// Enqueue inserts a new job.  If IdempotencyKey is set and a pending/running
// job with the same type+key already exists, it is silently skipped.
func (q *Queue) Enqueue(ctx context.Context, jobType string, payload any, opts EnqueueOptions) (*Job, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("workers: marshal payload: %w", err)
	}

	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	runAt := opts.RunAt
	if runAt.IsZero() {
		runAt = time.Now().UTC()
	}

	createdBy := opts.CreatedBy
	if createdBy == "" {
		createdBy = "system"
	}

	// If an idempotency key is supplied, check whether an active job already
	// exists before inserting.
	if opts.IdempotencyKey != nil {
		existing, err := q.findByIdempotencyKey(ctx, jobType, *opts.IdempotencyKey)
		if err != nil {
			return nil, err
		}

		if existing != nil {
			return existing, nil // already queued — idempotent
		}
	}

	row := q.db.QueryRow(ctx, `
		INSERT INTO jobs (
		    tenant_id, type, payload,
		    run_at, max_attempts,
		    idempotency_key, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+jobCols,
		opts.TenantID, jobType, payloadJSON,
		runAt, maxAttempts,
		opts.IdempotencyKey, createdBy,
	)

	job, err := scanJob(row)
	if err != nil {
		// Unique constraint violation on idempotency key — another goroutine
		// inserted first; treat as idempotent success.
		if isDuplicateError(err) {
			return q.findByIdempotencyKey(ctx, jobType, ptrStr(opts.IdempotencyKey))
		}
		return nil, fmt.Errorf("workers: enqueue: %w", err)
	}

	return job, nil
}

// ── Claim ─────────────────────────────────────────────────────────────────────

// Claim atomically claims the next available job for processing.
// It uses SELECT … FOR UPDATE SKIP LOCKED so concurrent workers never
// claim the same row.  Returns nil when no job is ready.
func (q *Queue) Claim(ctx context.Context, jobTypes []string) (*Job, error) {
	if len(jobTypes) == 0 {
		return nil, nil
	}

	// Build a parameterised IN clause.
	args := make([]any, len(jobTypes)+1)
	args[0] = time.Now().UTC()
	placeholders := make([]byte, 0, len(jobTypes)*5)

	for i, t := range jobTypes {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholder := fmt.Sprintf("$%d", i+2)
		placeholders = append(placeholders, placeholder...)
		args[i+1] = t
	}

	q2 := fmt.Sprintf(`
		UPDATE jobs
		SET    status     = 'running',
		       started_at = NOW(),
		       attempts   = attempts + 1,
		       updated_at = NOW()
		WHERE  id = (
		    SELECT id FROM jobs
		    WHERE  status  IN ('pending', 'failed')
		    AND    run_at  <= $1
		    AND    type    IN (%s)
		    AND    attempts < max_attempts
		    ORDER  BY run_at ASC
		    LIMIT  1
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING %s`, string(placeholders), jobCols)

	job, err := scanJob(q.db.QueryRow(ctx, q2, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("workers: claim: %w", err)
	}

	return job, nil
}

// ── Complete / Fail ───────────────────────────────────────────────────────────

// Complete marks a job as successfully completed.
func (q *Queue) Complete(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, `
		UPDATE jobs
		SET    status       = 'completed',
		       completed_at = NOW(),
		       updated_at   = NOW()
		WHERE  id = $1`, id)

	if err != nil {
		return fmt.Errorf("workers: complete job %s: %w", id, err)
	}

	return nil
}

// Fail records a job failure and either re-queues it (with backoff) or
// moves it to 'dead' if max_attempts has been reached.
func (q *Queue) Fail(ctx context.Context, id string, jobErr error) error {
	errMsg := jobErr.Error()

	_, err := q.db.Exec(ctx, `
		UPDATE jobs
		SET    last_error = $2,
		       status     = CASE
		           WHEN attempts >= max_attempts THEN 'dead'
		           ELSE 'failed'
		       END,
		       -- Exponential backoff: 2^attempts minutes, capped at 60 minutes.
		       run_at     = CASE
		           WHEN attempts >= max_attempts THEN run_at
		           ELSE NOW() + (LEAST(POWER(2, attempts), 60) * INTERVAL '1 minute')
		       END,
		       updated_at = NOW()
		WHERE  id = $1`, id, errMsg)

	if err != nil {
		return fmt.Errorf("workers: fail job %s: %w", id, err)
	}

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (q *Queue) findByIdempotencyKey(ctx context.Context, jobType, key string) (*Job, error) {
	row := q.db.QueryRow(ctx, `
		SELECT `+jobCols+`
		FROM   jobs
		WHERE  type = $1 AND idempotency_key = $2
		AND    status IN ('pending','running','failed')
		LIMIT  1`, jobType, key)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("workers: find by idempotency key: %w", err)
	}

	return job, nil
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*Job, error) {
	var j Job
	var status string
	var payloadRaw []byte

	err := row.Scan(
		&j.ID, &j.TenantID,
		&j.Type, &payloadRaw,
		&j.RunAt, &status, &j.Attempts, &j.MaxAttempts, &j.LastError,
		&j.StartedAt, &j.CompletedAt,
		&j.IdempotencyKey, &j.CreatedBy,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	j.Status = JobStatus(status)

	if err := json.Unmarshal(payloadRaw, &j.Payload); err != nil {
		j.Payload = map[string]any{}
	}

	return &j, nil
}

func isDuplicateError(err error) bool {
	return err != nil && fmt.Sprintf("%v", err) != "" &&
		containsStr(fmt.Sprintf("%v", err), "23505")
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
