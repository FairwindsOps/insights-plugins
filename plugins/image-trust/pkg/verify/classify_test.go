package verify

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestClassifyCosignFailure(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    models.Status
	}{
		{
			name:    "unsigned image",
			message: "Error: no matching signatures found",
			want:    models.StatusUnsigned,
		},
		{
			name:    "trusted identity mismatch",
			message: "Error: certificate identity https://github.com/other/repo did not match expected identities",
			want:    models.StatusSignedUntrusted,
		},
		{
			name:    "registry auth failure",
			message: "GET https://ghcr.io/v2/example/api/manifests/sha256:abc: UNAUTHORIZED",
			want:    models.StatusVerificationError,
		},
		{
			name:    "generic failure",
			message: "Error: context deadline exceeded",
			want:    models.StatusVerificationError,
		},
		{
			name:    "does not treat unrelated issuer text as untrusted",
			message: "Error: failed to reach issuer metadata endpoint: connection reset",
			want:    models.StatusVerificationError,
		},
		{
			name:    "does not treat unrelated 403 substring as auth failure",
			message: "Error: policy rule 4031 rejected the request",
			want:    models.StatusVerificationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := classifyCosignFailure(tt.message)
			require.Equal(t, tt.want, got)
		})
	}
}
