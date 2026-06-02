package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

const (
	// OutputTempFile is the staging path written before the final report is published.
	OutputTempFile = "/output/image-trust-temp.json"
	// OutputFile is the path the insights-uploader sidecar watches for datatype image-trust.
	OutputFile = "/output/image-trust.json"
)

// WriteFinalReport atomically publishes the report for the uploader sidecar.
func WriteFinalReport(report models.Report) error {
	if err := os.MkdirAll("/output", 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	return writeFinalReportAt(OutputTempFile, OutputFile, report)
}

func writeFinalReportAt(tempFile, finalFile string, report models.Report) error {
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		return fmt.Errorf("writing temp report: %w", err)
	}

	if err := os.Rename(tempFile, finalFile); err != nil {
		return fmt.Errorf("renaming report: %w", err)
	}
	return nil
}
