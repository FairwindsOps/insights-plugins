package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

const (
	TempDir         = "/output/tmp"
	FinalReportPath = TempDir + "/final-report.json"
)

// WriteReport writes the report to disk atomically.
func WriteReport(path string, report models.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), "image-trust-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("writing temp report: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp report: %w", err)
	}

	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("renaming report: %w", err)
	}
	return nil
}
