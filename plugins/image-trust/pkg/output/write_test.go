package output

import (
	"path/filepath"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestWriteReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	err := WriteReport(path, models.Report{
		Images: []models.ImageTrustResult{{Name: "example", Status: models.StatusUnknown}},
	})
	require.NoError(t, err)

	require.FileExists(t, path)
}
