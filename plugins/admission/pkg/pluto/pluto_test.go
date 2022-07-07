package pluto

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fairwindsops/pluto/v5/pkg/api"
	"github.com/stretchr/testify/require"
)

// plutoReportItems represents a slice of the upstream Pluto api.Output
// struct, used to unmarshal the Pluto JSON output format for examination within
// tests.
type plutoReportItems struct {
	Items []api.Output
}

const ingressV1Beta1Manifest = `
# THIs ingress is deprecated in Kube 1.21 and removed in 1.22.
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: test
spec:
  rules:
  - host: test.domain.com
    http:
      paths:
      - path: /
        backend:
          serviceName: test
          servicePort: 80
`

func TestParsePlutoTargetVersions(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description string
		input       string // targetVersions to be parsed
		want        map[string]string
		expectError bool
	}{
		{
			description: "single component k8s=v1.21.0",
			input:       "k8s=v1.21.0",
			want:        map[string]string{"k8s": "v1.21.0"},
		},
		{
			description: "invalid single component k8s=1.21.0 lacking the leading v",
			input:       "k8s=1.21.0",
			expectError: true,
		},
		{
			description: "multiple components k8s=v1.23.0 cert-manager=v1.8.0 istio=v1.14.0",
			input:       "k8s=v1.23.0,cert-manager=v1.8.0,istio=v1.14.0",
			want:        map[string]string{"k8s": "v1.23.0", "cert-manager": "v1.8.0", "istio": "v1.14.0"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			got, err := ParsePlutoTargetVersions(tc.input)
			if tc.expectError {
				require.Error(t, err)
			}
			if !tc.expectError {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDeprecatedIngressV1Beta1WithKube121(t *testing.T) {
	t.Parallel()
	input := []byte(ingressV1Beta1Manifest)
	plutoReportData, err := ProcessPluto(input, map[string]string{"k8s": "v1.21.0"})
	require.NoError(t, err)
	var plutoReport plutoReportItems
	err = json.Unmarshal(plutoReportData.Contents, &plutoReport)
	require.NoError(t, err, fmt.Sprintf("unmarshaling pluto contents from json: %v", string(plutoReportData.Contents)))
	require.Equal(t, 1, len(plutoReport.Items), fmt.Sprintf("The Pluto output should have returned 1 item for a deprecated V1Beta1 Ingress. The full pluto output is: %v", string(plutoReportData.Contents)))
	require.Equal(t, true, plutoReport.Items[0].Deprecated, fmt.Sprintf("Pluto should have returned that this API is deprecated. The full pluto output is: %v", string(plutoReportData.Contents)))
	require.Equal(t, false, plutoReport.Items[0].Removed, fmt.Sprintf("Pluto should have returned that this API is not removed. The full pluto output is: %v", string(plutoReportData.Contents)))
}

func TestDeprecatedAndRemovedIngressV1Beta1WithKube123(t *testing.T) {
	t.Parallel()
	input := []byte(ingressV1Beta1Manifest)
	plutoReportData, err := ProcessPluto(input, map[string]string{"k8s": "v1.23.0"})
	require.NoError(t, err)
	var plutoReport plutoReportItems
	err = json.Unmarshal(plutoReportData.Contents, &plutoReport)
	require.NoError(t, err, fmt.Sprintf("unmarshaling pluto contents from json: %v", string(plutoReportData.Contents)))
	require.Equal(t, 1, len(plutoReport.Items), fmt.Sprintf("The Pluto output should have returned 1 item for a deprecated V1Beta1 Ingress. The full pluto output is: %v", string(plutoReportData.Contents)))
	require.Equal(t, true, plutoReport.Items[0].Deprecated, fmt.Sprintf("Pluto should have returned that this API is deprecated. The full pluto output is: %v", string(plutoReportData.Contents)))
	require.Equal(t, true, plutoReport.Items[0].Removed, fmt.Sprintf("Pluto should have returned that this API is not removed. The full pluto output is: %v", string(plutoReportData.Contents)))
}
