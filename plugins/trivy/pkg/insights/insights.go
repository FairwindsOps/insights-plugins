package insights

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
)

// FetchLastReport returns the last report for Trivy from Fairwinds Insights
func FetchLastReport(ctx context.Context, host, org, cluster, token string) (*models.MinimizedReport, error) {
	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/trivy/latest.json", host, org, cluster)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return &models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad Status code on get last report: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return util.UnmarshalAndFixReport(body)
}
