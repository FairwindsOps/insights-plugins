package insights

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/imroc/req/v3"
)

type Client interface {
	GetClusterKyvernoPoliciesYAML() (string, error)
}

func NewClient(host, token, organization, cluster string, devMode bool) Client {
	if os.Getenv("MOCK_INSIGHTS_CLIENT") == "true" {
		slog.Info("Using mock insights client")
		return &MockClient{}
	}

	commonHeaders := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}
	client := req.C().
		SetBaseURL(host).
		SetCommonHeaders(commonHeaders).
		SetTimeout(10*time.Second).
		SetCommonRetryBackoffInterval(1*time.Second, 5*time.Second).
		SetCommonRetryCount(3)

	if devMode {
		slog.Info("running HTTP Client in development mode")
		client.DevMode()
	}

	return HTTPClient{organization: organization, cluster: cluster, client: client}
}

type HTTPClient struct {
	organization, cluster string
	client                *req.Client
}

func (c HTTPClient) GetClusterKyvernoPoliciesYAML() (string, error) {
	slog.Debug("Getting cluster Kyverno policies YAML", "organization", c.organization, "cluster", c.cluster)
	url := fmt.Sprintf("/v0/organizations/%s/clusters/%s/kyverno-policies/with-app-groups-applied/yaml", c.organization, c.cluster)
	resp, err := c.client.R().Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster Kyverno policies: %w", err)
	}

	if resp.IsErrorState() {
		return "", fmt.Errorf("error getting cluster Kyverno policies: status %d, body %s", resp.StatusCode, resp.String())
	}

	return resp.String(), nil
}

type MockClient struct{}

func (m MockClient) GetClusterKyvernoPoliciesYAML() (string, error) {
	slog.Info("Mock: Getting cluster Kyverno policies YAML")
	return `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: mock-policy
  annotations:
    insights.fairwinds.com/owned-by: "Fairwinds Insights"
spec:
  validationFailureAction: enforce
  rules:
  - name: mock-rule
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "This is a mock policy"
      pattern:
        spec:
          containers:
          - name: "*"
            image: "!*:latest"`, nil
}
