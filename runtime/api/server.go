package api

import (
	"context"
	"net/http"
	"time"
)

// Server represents the HTTP server for the runtime sidecar API.
type Server struct {
	store      *RunStore
	executor   TaskExecutorFunc
	httpServer *http.Server
	handlers   *Handlers
}

// NewServer creates a new Server instance.
func NewServer(addr string, executor TaskExecutorFunc) *Server {
	store := NewRunStore()
	handlers := NewHandlers(store, executor)

	mux := http.NewServeMux()

	// Register routes using Go 1.22+ method routing
	mux.HandleFunc("POST /api/v1/runs", handlers.HandleStartRun)
	mux.HandleFunc("GET /api/v1/runs/{id}", handlers.HandleGetStatus)
	mux.HandleFunc("POST /api/v1/runs/{id}/abort", handlers.HandleAbort)
	mux.HandleFunc("POST /api/v1/runs/{id}/tasks", handlers.HandleEnqueueTask)

	return &Server{
		store:    store,
		executor: executor,
		handlers: handlers,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start starts the HTTP server.
// Blocks until the server is stopped or an error occurs.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
// Cancels all active runs and waits for them to complete before shutting down HTTP.
func (s *Server) Shutdown(ctx context.Context) error {
	// Cancel all active runs
	cancelled := s.store.CancelAll()
	if cancelled > 0 {
		// Wait for runs to complete (use half the context deadline for this)
		deadline, ok := ctx.Deadline()
		if ok {
			waitTimeout := time.Until(deadline) / 2
			if waitTimeout > 0 {
				s.store.WaitAll(waitTimeout)
			}
		}
	}

	return s.httpServer.Shutdown(ctx)
}

// Store returns the RunStore for testing purposes.
func (s *Server) Store() *RunStore {
	return s.store
}

// Handlers returns the Handlers for testing purposes.
func (s *Server) Handlers() *Handlers {
	return s.handlers
}
