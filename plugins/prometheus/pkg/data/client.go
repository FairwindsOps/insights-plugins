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

	"github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	p8sConfig "github.com/prometheus/common/config"
)

// TenantHeader is the HTTP header used by Grafana Mimir and other multi-tenant
// Prometheus-compatible backends to identify the tenant.
const TenantHeader = "X-Scope-OrgID"

// clientConfig holds configuration options for creating a Prometheus client.
type clientConfig struct {
	bearerToken string
	tenantID    string
}

// ClientOption is a functional option for configuring the Prometheus client.
type ClientOption func(*clientConfig)

// WithBearerToken sets the bearer token for authentication.
// This is equivalent to the bearerToken parameter in the original GetClient function.
func WithBearerToken(token string) ClientOption {
	return func(c *clientConfig) {
		c.bearerToken = token
	}
}

// WithTenantID sets the tenant ID for multi-tenant Prometheus backends like Grafana Mimir.
// When set, the X-Scope-OrgID header will be included in all requests.
func WithTenantID(tenantID string) ClientOption {
	return func(c *clientConfig) {
		c.tenantID = tenantID
	}
}

// tenantRoundTripper is an http.RoundTripper that injects the tenant header
// into all requests for multi-tenant Prometheus backends like Grafana Mimir.
type tenantRoundTripper struct {
	tenantID string
	next     http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (t *tenantRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original
	reqClone := req.Clone(req.Context())
	reqClone.Header.Set(TenantHeader, t.tenantID)
	return t.next.RoundTrip(reqClone)
}

// newTenantRoundTripper creates a new RoundTripper that injects the tenant header.
func newTenantRoundTripper(tenantID string, next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &tenantRoundTripper{
		tenantID: tenantID,
		next:     next,
	}
}

// GetClientWithOptions returns a Prometheus API client for a given address with
// configurable options. This function supports multi-tenant Prometheus backends
// like Grafana Mimir through the WithTenantID option.
//
// For Mimir deployments, the address should include the full path to the Prometheus
// API endpoint (e.g., "https://mimir.example.com/prometheus"). The Prometheus client
// will automatically append /api/v1 for API calls.
//
// Example usage:
//
//	// Standard Prometheus
//	client, err := GetClientWithOptions("http://prometheus:9090")
//
//	// Prometheus with bearer token
//	client, err := GetClientWithOptions("http://prometheus:9090",
//	    WithBearerToken("my-token"))
//
//	// Grafana Mimir with tenant ID
//	client, err := GetClientWithOptions("https://mimir.example.com/prometheus",
//	    WithTenantID("my-tenant"))
//
//	// Grafana Mimir with tenant ID and bearer token
//	client, err := GetClientWithOptions("https://mimir.example.com/prometheus",
//	    WithBearerToken("my-token"),
//	    WithTenantID("my-tenant"))
func GetClientWithOptions(address string, opts ...ClientOption) (prometheusV1.API, error) {
	cfg := &clientConfig{}

	// Apply all options
	for _, opt := range opts {
		opt(cfg)
	}

	config := api.Config{
		Address: address,
	}

	// Build the RoundTripper chain
	var rt http.RoundTripper = api.DefaultRoundTripper

	// Add bearer token authentication if provided
	if cfg.bearerToken != "" {
		rt = p8sConfig.NewAuthorizationCredentialsRoundTripper(
			"Bearer",
			p8sConfig.NewInlineSecret(cfg.bearerToken),
			rt,
		)
	}

	// Add tenant header injection if tenant ID is provided
	if cfg.tenantID != "" {
		rt = newTenantRoundTripper(cfg.tenantID, rt)
	}

	config.RoundTripper = rt

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return prometheusV1.NewAPI(client), nil
}

