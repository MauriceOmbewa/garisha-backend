package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	// readTimeout caps the time allowed to read the full request including body.
	readTimeout = 10 * time.Second

	// writeTimeout caps the time allowed to write the response.
	writeTimeout = 30 * time.Second

	// idleTimeout caps how long an idle keep-alive connection is kept open.
	idleTimeout = 60 * time.Second

	// shutdownTimeout is the maximum time we wait for in-flight requests to
	// finish before forcibly closing the server.
	shutdownTimeout = 15 * time.Second
)

// Server wraps *http.Server and adds a graceful shutdown helper.
type Server struct {
	http *http.Server
	log  *slog.Logger
}

// NewServer creates an *http.Server configured with production-grade timeouts.
func NewServer(port string, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		log: log,
		http: &http.Server{
			Addr:         fmt.Sprintf(":%s", port),
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
	}
}

// Start begins listening for incoming connections.  It blocks until the
// server returns an error (other than http.ErrServerClosed).
func (s *Server) Start() error {
	s.log.Info("http server listening", "addr", s.http.Addr)

	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen: %w", err)
	}

	return nil
}

// Shutdown gracefully drains in-flight requests within shutdownTimeout then
// closes the server.
func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	s.log.Info("shutting down http server", "timeout_s", shutdownTimeout.Seconds())

	if err := s.http.Shutdown(ctx); err != nil {
		s.log.Error("server shutdown error", "error", err)
	}
}
