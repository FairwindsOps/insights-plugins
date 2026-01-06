// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantRoundTripper(t *testing.T) {
	testTenantID := "my-test-tenant"

	// Track headers received by the mock server
	var receivedHeaders http.Header

	// Create a test server that captures request headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return minimal valid Prometheus API response
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Create client with tenant ID
	client, err := GetClientWithOptions(server.URL, WithTenantID(testTenantID))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Make a request (any query will do)
	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify the tenant header was set
	assert.Equal(t, testTenantID, receivedHeaders.Get(TenantHeader))
}

func TestTenantRoundTripperWithBearerToken(t *testing.T) {
	testTenantID := "my-test-tenant"
	testBearerToken := "my-secret-token"

	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Create client with both tenant ID and bearer token
	client, err := GetClientWithOptions(server.URL,
		WithBearerToken(testBearerToken),
		WithTenantID(testTenantID),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify both headers are set
	assert.Equal(t, testTenantID, receivedHeaders.Get(TenantHeader))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Bearer "+testBearerToken)
}

func TestClientWithoutTenantID(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Create client without tenant ID
	client, err := GetClientWithOptions(server.URL)
	require.NoError(t, err)
	require.NotNil(t, client)

	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify the tenant header is NOT set
	assert.Empty(t, receivedHeaders.Get(TenantHeader))
}

func TestGetClientBackwardCompatibility(t *testing.T) {
	testBearerToken := "my-secret-token"

	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Use the original GetClient function
	client, err := GetClient(server.URL, testBearerToken)
	require.NoError(t, err)
	require.NotNil(t, client)

	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify bearer token is set and tenant header is NOT set
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Bearer "+testBearerToken)
	assert.Empty(t, receivedHeaders.Get(TenantHeader))
}

func TestGetClientWithEmptyBearerToken(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Use the original GetClient function with empty bearer token
	client, err := GetClient(server.URL, "")
	require.NoError(t, err)
	require.NotNil(t, client)

	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify no Authorization header is set
	assert.Empty(t, receivedHeaders.Get("Authorization"))
}

func TestWithBearerTokenOption(t *testing.T) {
	cfg := &clientConfig{}
	opt := WithBearerToken("test-token")
	opt(cfg)
	assert.Equal(t, "test-token", cfg.bearerToken)
}

func TestWithTenantIDOption(t *testing.T) {
	cfg := &clientConfig{}
	opt := WithTenantID("test-tenant")
	opt(cfg)
	assert.Equal(t, "test-tenant", cfg.tenantID)
}

func TestNewTenantRoundTripperWithNilNext(t *testing.T) {
	rt := newTenantRoundTripper("test-tenant", nil)
	trt, ok := rt.(*tenantRoundTripper)
	require.True(t, ok)
	assert.Equal(t, "test-tenant", trt.tenantID)
	assert.Equal(t, http.DefaultTransport, trt.next)
}

// TestMimirPathHandling demonstrates that the Prometheus client correctly handles
// paths like /prometheus for Mimir deployments. The client appends /api/v1 to
// the address, so users should provide the base path without /api/v1.
func TestMimirPathHandling(t *testing.T) {
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()

	// Simulate Mimir-style address with /prometheus path
	// In real usage, this would be something like https://mimir.example.com/prometheus
	// but for testing, we append /prometheus to our test server URL
	mimirStyleURL := server.URL + "/prometheus"

	client, err := GetClientWithOptions(mimirStyleURL, WithTenantID("test-tenant"))
	require.NoError(t, err)

	_, _, err = client.Query(t.Context(), "up", time.Now())
	require.NoError(t, err)

	// Verify the path includes both /prometheus and /api/v1/query
	// The Prometheus client library appends /api/v1/query to the base address
	assert.Equal(t, "/prometheus/api/v1/query", requestPath)
}

