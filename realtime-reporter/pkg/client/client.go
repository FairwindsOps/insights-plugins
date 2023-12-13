package client

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	maxTries  = 3
	maxDelay  = 30 * time.Second
	maxJitter = 10 * time.Millisecond
)

// TODO: this doesn't send a request to
// apiURL = fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental/failure", host, organization, cluster, reportType)
// on failure, need to figure out how to do this with retry-go
func UploadToInsights(timestamp int64, reportType string, payload []byte) error {
	var resp *http.Response
	host := viper.GetString("host")
	organization := viper.GetString("organization")
	cluster := viper.GetString("cluster")
	token := viper.GetString("token")

	err := retry.Do(
		func() error {
			var err error
			apiURL := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental", host, organization, cluster, reportType)

			req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
			if err != nil {
				return fmt.Errorf("error creating HTTP request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Fairwinds-Event-Timestamp", fmt.Sprintf("%d", timestamp))
			req.Header.Set("Authorization", "Bearer "+token)

			// create an HTTP client and send the request
			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			return err
		},
		retry.Attempts(maxTries),
		retry.OnRetry(func(n uint, err error) {
			logrus.Infof("retrying request after error: %v", err)
		}),
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			logrus.Infof("server fails with: %v", err.Error())
			// apply a default exponential back off strategy
			return retry.BackOffDelay(n, err, config)
		}),
		// apply random jitter
		retry.MaxJitter(maxJitter),
	)

	return err
}
