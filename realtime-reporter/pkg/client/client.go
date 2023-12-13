package client

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	maxTries = 3
)

func UploadToInsights(timestamp int64, reportType string, payload []byte) error {
	host := viper.GetString("host")
	organization := viper.GetString("organization")
	cluster := viper.GetString("cluster")
	token := viper.GetString("token")

	var sendError bool
	var tries int

	for {
		apiURL := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental", host, organization, cluster, reportType)
		if sendError {
			apiURL = fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental/failure", host, organization, cluster, reportType)
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
		if err != nil {
			return fmt.Errorf("error creating HTTP request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fairwinds-Event-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("Authorization", "Bearer "+token) // Add your authentication token if needed

		// Create an HTTP client and send the request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("error making HTTP request: %v", err)
		}
		defer resp.Body.Close()

		// Check the response
		if resp.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Error("Failed to upload event - status code:", resp.StatusCode)

			if sendError {
				return fmt.Errorf("failed to upload event - status code: %d - tried %d times", resp.StatusCode, tries-1) // failed to upload error
			}

			if tries >= maxTries {
				time.Sleep(100 * time.Millisecond)
				sendError = true
			}
			logrus.Warnf("failed to upload event - status code: %d - trying again...[%d/%d]", resp.StatusCode, tries, maxTries)
			tries++
		}
	}

	return nil
}
