package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the health status of the application
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusStarting  HealthStatus = "starting"
	StatusStopping  HealthStatus = "stopping"
)

// HealthResponse represents the response from health check endpoints
type HealthResponse struct {
	Status    HealthStatus   `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Uptime    string         `json:"uptime"`
	Version   string         `json:"version,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// HealthChecker defines the interface for health check components
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
	GetName() string
}

// Server provides HTTP health check endpoints
type Server struct {
	server       *http.Server
	status       HealthStatus
	startTime    time.Time
	version      string
	checkers     []HealthChecker
	mu           sync.RWMutex
	shutdownCh   chan struct{}
	shutdownDone chan struct{}
}

// NewServer creates a new health check server
func NewServer(port int, version string) *Server {
	mux := http.NewServeMux()

	server := &Server{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
		status:       StatusStarting,
		startTime:    time.Now(),
		version:      version,
		checkers:     make([]HealthChecker, 0),
		shutdownCh:   make(chan struct{}),
		shutdownDone: make(chan struct{}),
	}

	// Register health check endpoints
	mux.HandleFunc("/healthz", server.livenessHandler)
	mux.HandleFunc("/readyz", server.readinessHandler)
	mux.HandleFunc("/health", server.healthHandler)

	return server
}

// RegisterChecker adds a health checker to the server
func (s *Server) RegisterChecker(checker HealthChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers = append(s.checkers, checker)
}

// SetStatus updates the overall health status
func (s *Server) SetStatus(status HealthStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// GetStatus returns the current health status
func (s *Server) GetStatus() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Start starts the health check server
func (s *Server) Start() error {
	s.SetStatus(StatusHealthy)

	go func() {
		slog.Info("Starting health check server", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health check server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the health check server
func (s *Server) Stop(ctx context.Context) error {
	s.SetStatus(StatusStopping)

	slog.Info("Stopping health check server")

	// Shutdown the HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown health check server", "error", err)
		return err
	}

	close(s.shutdownDone)
	slog.Info("Health check server stopped")
	return nil
}

// livenessHandler handles Kubernetes liveness probes
func (s *Server) livenessHandler(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()

	// Liveness check - if the process is running, it's alive
	// Only return unhealthy if we're in a stopping state
	if status == StatusStopping {
		s.writeHealthResponse(w, StatusStopping, http.StatusServiceUnavailable)
		return
	}

	// For liveness, we return the actual status but with OK HTTP status
	// This allows monitoring systems to see the actual health status
	s.writeHealthResponse(w, status, http.StatusOK)
}

// readinessHandler handles Kubernetes readiness probes
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()

	// Readiness check - only ready if healthy and not stopping
	if status != StatusHealthy {
		s.writeHealthResponse(w, status, http.StatusServiceUnavailable)
		return
	}

	// Check all registered health checkers
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	details := make(map[string]any)
	allHealthy := true

	for _, checker := range s.checkers {
		if err := checker.CheckHealth(ctx); err != nil {
			details[checker.GetName()] = map[string]any{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			allHealthy = false
		} else {
			details[checker.GetName()] = map[string]any{
				"status": "healthy",
			}
		}
	}

	if allHealthy {
		s.writeHealthResponse(w, StatusHealthy, http.StatusOK, details)
	} else {
		s.writeHealthResponse(w, StatusUnhealthy, http.StatusServiceUnavailable, details)
	}
}

// healthHandler provides detailed health information
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()
	details := make(map[string]any)

	// Check all registered health checkers
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for _, checker := range s.checkers {
		if err := checker.CheckHealth(ctx); err != nil {
			details[checker.GetName()] = map[string]any{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		} else {
			details[checker.GetName()] = map[string]any{
				"status": "healthy",
			}
		}
	}

	s.writeHealthResponse(w, status, http.StatusOK, details)
}

// writeHealthResponse writes a health response to the HTTP response writer
func (s *Server) writeHealthResponse(w http.ResponseWriter, status HealthStatus, httpStatus int, details ...map[string]any) {
	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Uptime:    time.Since(s.startTime).String(),
		Version:   s.version,
	}

	if len(details) > 0 {
		response.Details = details[0]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode health response", "error", err)
	}
}

// WaitForShutdown waits for the server to be shut down
func (s *Server) WaitForShutdown() {
	<-s.shutdownDone
}
