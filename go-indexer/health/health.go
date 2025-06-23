package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/timewave/vault-indexer/go-indexer/logger"
)

// Server represents a health check HTTP server
type Server struct {
	port   int
	server *http.Server
	logger *logger.Logger
	status string
	label  string
	mu     sync.RWMutex
}

// NewServer creates a new health check server
func NewServer(port int, label string) *Server {
	s := &Server{
		port:   port,
		logger: logger.NewLogger("HealthCheck"),
		status: "healthy",
		label:  label,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return s
}

// Start starts the health check server
func (s *Server) Start() error {
	s.logger.Info("Starting health check server on port %d for %s", s.port, s.label)

	// Create a listener first to check if we can bind to the port
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind to port %d: %w", s.port, err)
	}
	s.SetStatus("healthy")

	// Start the server in a goroutine
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Health check server error: %v", err)
			s.SetStatus("unhealthy")
		}
	}()
	return nil
}

// Stop gracefully stops the health check server
func (s *Server) Stop() error {
	s.logger.Info("Stopping health check server for %s", s.label)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// SetStatus updates the health status
func (s *Server) SetStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// GetStatus returns the current health status
func (s *Server) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if status == "healthy" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"%s","service":"%s"}`, status, s.label)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"%s","service":"%s"}`, status, s.label)
	}
}
