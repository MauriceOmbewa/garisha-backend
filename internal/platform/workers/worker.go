package workers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Worker is a goroutine-based worker pool that polls the job queue and
// dispatches jobs to registered handlers.
type Worker struct {
	queue    *Queue
	registry Registry
	log      *slog.Logger

	concurrency int           // number of goroutines
	pollInterval time.Duration // how often to check for new jobs

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Config holds Worker configuration.
type Config struct {
	// Concurrency is the number of parallel worker goroutines.  Default: 3.
	Concurrency int

	// PollInterval is how often idle workers poll for new jobs.  Default: 5s.
	PollInterval time.Duration
}

// NewWorker creates a Worker.
func NewWorker(queue *Queue, registry Registry, log *slog.Logger, cfg Config) *Worker {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 3
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Worker{
		queue:        queue,
		registry:     registry,
		log:          log,
		concurrency:  concurrency,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start launches the worker pool as background goroutines.
// It is non-blocking — the caller's goroutine is not occupied.
// Call Stop() to drain and shut down.
func (w *Worker) Start(ctx context.Context) {
	w.log.Info("workers: starting pool",
		"concurrency",   w.concurrency,
		"poll_interval", w.pollInterval,
	)

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.run(ctx, i)
	}
}

// Stop signals all workers to stop and blocks until in-flight jobs complete.
func (w *Worker) Stop() {
	w.log.Info("workers: stopping pool")
	close(w.stopCh)
	w.wg.Wait()
	w.log.Info("workers: all goroutines stopped")
}

// run is the main loop for a single worker goroutine.
func (w *Worker) run(ctx context.Context, id int) {
	defer w.wg.Done()

	log := w.log.With("worker_id", id)
	log.Debug("worker started")

	// Build the list of job types this pool can handle.
	jobTypes := make([]string, 0, len(w.registry))
	for t := range w.registry {
		jobTypes = append(jobTypes, t)
	}

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			log.Debug("worker stopping")
			return

		case <-ctx.Done():
			log.Debug("worker context cancelled")
			return

		case <-ticker.C:
			// Drain all available jobs before waiting again.
			for {
				claimed, err := w.queue.Claim(ctx, jobTypes)
				if err != nil {
					log.Error("workers: claim error", "error", err)
					break
				}

				if claimed == nil {
					break // nothing ready — go back to sleep
				}

				w.dispatch(ctx, claimed, log)
			}
		}
	}
}

// dispatch executes a job using its registered handler.
func (w *Worker) dispatch(ctx context.Context, job *Job, log *slog.Logger) {
	handler, ok := w.registry[job.Type]
	if !ok {
		log.Error("workers: no handler registered",
			"job_id", job.ID,
			"type",   job.Type,
		)
		errMsg := fmt.Sprintf("no handler registered for job type %q", job.Type)
		_ = w.queue.Fail(ctx, job.ID, fmt.Errorf("%s", errMsg))
		return
	}

	start := time.Now()
	log.Info("workers: executing job",
		"job_id",   job.ID,
		"type",     job.Type,
		"attempts", job.Attempts,
	)

	// Execute with a per-job timeout of 2 minutes.
	jobCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	err := handler(jobCtx, job)
	elapsed := time.Since(start)

	if err != nil {
		log.Warn("workers: job failed",
			"job_id",   job.ID,
			"type",     job.Type,
			"attempts", job.Attempts,
			"elapsed",  elapsed,
			"error",    err,
		)

		if failErr := w.queue.Fail(ctx, job.ID, err); failErr != nil {
			log.Error("workers: failed to record job failure",
				"job_id", job.ID,
				"error",  failErr,
			)
		}

		return
	}

	log.Info("workers: job completed",
		"job_id",  job.ID,
		"type",    job.Type,
		"elapsed", elapsed,
	)

	if completeErr := w.queue.Complete(ctx, job.ID); completeErr != nil {
		log.Error("workers: failed to mark job complete",
			"job_id", job.ID,
			"error",  completeErr,
		)
	}
}
