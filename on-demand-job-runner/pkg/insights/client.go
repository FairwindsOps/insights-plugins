package insights

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/imroc/req/v3"
)

type Client interface {
	ClaimOnDemandJobs(limit int) ([]OnDemandJob, error)
	UpdateOnDemandJobStatus(jobID int64, status OnDemandJobStatus) error
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

func (c HTTPClient) ClaimOnDemandJobs(limit int) ([]OnDemandJob, error) {
	slog.Info("Claiming on-demand jobs", "organization", c.organization, "cluster", c.cluster)
	url := fmt.Sprintf("/v0/organizations/%s/clusters/%s/reports/on-demand-jobs/claim?limit=1", c.organization, c.cluster)
	resp, err := c.client.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query on-demand jobs: %w", err)
	}

	if resp.IsErrorState() {
		return nil, fmt.Errorf("error querying on-demand jobs: status %d, body %s", resp.StatusCode, resp.String())
	}

	slog.Info("Successfully queried on-demand jobs", "status", resp.StatusCode, "body", resp.String())
	var onDemandJobs []OnDemandJob
	if err := resp.UnmarshalJson(&onDemandJobs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal on-demand jobs response: %w", err)
	}

	return onDemandJobs, nil
}

func (c HTTPClient) UpdateOnDemandJobStatus(jobID int64, status OnDemandJobStatus) error {
	url := fmt.Sprintf("/v0/organizations/%s/clusters/%s/reports/on-demand-jobs/%d/status", c.organization, c.cluster, jobID)
	resp, err := c.client.R().SetBody(map[string]string{"status": string(status)}).Patch(url)
	if err != nil {
		return fmt.Errorf("failed to update on-demand job status: %w", err)
	}

	if resp.IsErrorState() {
		return fmt.Errorf("error updating on-demand job status: status %d, body %s", resp.StatusCode, resp.String())
	}
	return nil
}

type MockClient struct{}

func (m MockClient) ClaimOnDemandJobs(limit int) ([]OnDemandJob, error) {
	slog.Info("Mock: Claiming on-demand jobs", "limit", limit)
	return []OnDemandJob{
		{
			ID:         1,
			ReportType: "trivy",
			Status:     string(JobStatusPending),
			Options: map[string]string{
				"imagesToScan": "quay.io/fairwinds/polaris:9.6,quay.io/fairwinds/workloads:2.6",
			},
		},
	}, nil
}

func (m MockClient) UpdateOnDemandJobStatus(jobID int64, status OnDemandJobStatus) error {
	slog.Info("Mock: Updating on-demand job status", "jobID", jobID, "status", status)
	return nil
}
