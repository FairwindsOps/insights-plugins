package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHealthChecker is a mock implementation of HealthChecker for testing
type mockHealthChecker struct {
	name    string
	healthy bool
	error   error
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context) error {
	if m.error != nil {
		return m.error
	}
	if !m.healthy {
		return assert.AnError
	}
	return nil
}

func (m *mockHealthChecker) GetName() string {
	return m.name
}

func TestNewServer(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	assert.NotNil(t, server)
	assert.Equal(t, StatusStarting, server.GetStatus())
	assert.Equal(t, ":8080", server.server.Addr)
}

func TestServer_SetStatus(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	
	server.SetStatus(StatusHealthy)
	assert.Equal(t, StatusHealthy, server.GetStatus())
	
	server.SetStatus(StatusUnhealthy)
	assert.Equal(t, StatusUnhealthy, server.GetStatus())
}

func TestServer_RegisterChecker(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	checker := &mockHealthChecker{name: "test-checker", healthy: true}
	
	server.RegisterChecker(checker)
	
	// Verify checker was registered by checking readiness
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	
	server.SetStatus(StatusHealthy)
	server.readinessHandler(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLivenessHandler(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	
	tests := []struct {
		name           string
		status         HealthStatus
		expectedStatus int
	}{
		{
			name:           "healthy status",
			status:         StatusHealthy,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unhealthy status",
			status:         StatusUnhealthy,
			expectedStatus: http.StatusOK, // Liveness should still return OK if process is running
		},
		{
			name:           "stopping status",
			status:         StatusStopping,
			expectedStatus: http.StatusServiceUnavailable,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.SetStatus(tt.status)
			
			req := httptest.NewRequest("GET", "/healthz", nil)
			w := httptest.NewRecorder()
			
			server.livenessHandler(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response HealthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.status, response.Status)
		})
	}
}

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name           string
		status         HealthStatus
		checkers       []HealthChecker
		expectedStatus int
	}{
		{
			name:           "healthy with no checkers",
			status:         StatusHealthy,
			checkers:       []HealthChecker{},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "healthy with healthy checkers",
			status: StatusHealthy,
			checkers: []HealthChecker{
				&mockHealthChecker{name: "checker1", healthy: true},
				&mockHealthChecker{name: "checker2", healthy: true},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "healthy with unhealthy checkers",
			status: StatusHealthy,
			checkers: []HealthChecker{
				&mockHealthChecker{name: "checker1", healthy: true},
				&mockHealthChecker{name: "checker2", healthy: false},
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "unhealthy status",
			status:         StatusUnhealthy,
			checkers:       []HealthChecker{},
			expectedStatus: http.StatusServiceUnavailable,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new server for each test to avoid state pollution
			testServer := NewServer(8080, "1.0.0")
			testServer.SetStatus(tt.status)
			
			// Register checkers
			for _, checker := range tt.checkers {
				testServer.RegisterChecker(checker)
			}
			
			req := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()
			
			testServer.readinessHandler(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response HealthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, StatusHealthy, response.Status)
			} else {
				assert.Equal(t, StatusUnhealthy, response.Status)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	server.SetStatus(StatusHealthy)
	
	// Register a healthy checker
	checker := &mockHealthChecker{name: "test-checker", healthy: true}
	server.RegisterChecker(checker)
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	server.healthHandler(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response HealthResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	assert.Equal(t, StatusHealthy, response.Status)
	assert.NotEmpty(t, response.Details)
	assert.Contains(t, response.Details, "test-checker")
}

func TestServer_Stop(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	server.SetStatus(StatusHealthy)
	
	// Start server in a goroutine
	go func() {
		server.Start()
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Stop server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := server.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, StatusStopping, server.GetStatus())
}

func TestHealthResponse_JSON(t *testing.T) {
	server := NewServer(8080, "1.0.0")
	server.SetStatus(StatusHealthy)
	
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	
	server.livenessHandler(w, req)
	
	var response HealthResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	assert.Equal(t, StatusHealthy, response.Status)
	assert.NotZero(t, response.Timestamp)
	assert.NotEmpty(t, response.Uptime)
	assert.Equal(t, "1.0.0", response.Version)
}
