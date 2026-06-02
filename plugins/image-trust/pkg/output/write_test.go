package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestWriteFinalReportAt(t *testing.T) {
	dir := t.TempDir()
	tempFile := filepath.Join(dir, "image-trust-temp.json")
	finalFile := filepath.Join(dir, "image-trust.json")

	err := writeFinalReportAt(tempFile, finalFile, models.Report{
		Images: []models.ImageTrustResult{{
			Name:        "example",
			Status:      models.StatusUnknown,
			Allowlisted: false,
			Owners:      []models.Resource{},
		}},
	})
	require.NoError(t, err)
	require.FileExists(t, finalFile)
	require.NoFileExists(t, tempFile)

	info, err := os.Stat(finalFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	data, err := os.ReadFile(finalFile)
	require.NoError(t, err)
	var decoded models.Report
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Images, 1)
}
