package client

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	httpretryafter "github.com/aereal/go-httpretryafter"
	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	maxTries  = 10
	minDelay  = 100 * time.Millisecond
	maxDelay  = 30 * time.Second
	maxJitter = 50 * time.Millisecond
)

type RetryAfterError struct {
	response http.Response
}

func (err RetryAfterError) Error() string {
	return fmt.Sprintf(
		"Request to %s fail %s (%d)",
		err.response.Request.RequestURI,
		err.response.Status,
		err.response.StatusCode,
	)
}

type SomeOtherError struct {
	err        string
	retryAfter time.Duration
}

func (err SomeOtherError) Error() string {
	return err.err
}

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
		retry.Delay(minDelay),
		retry.MaxDelay(maxDelay),
		retry.MaxJitter(maxJitter),
		retry.OnRetry(func(n uint, err error) {
			logrus.Infof("retrying request after error: %v", err)
		}),
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			switch e := err.(type) {
			case RetryAfterError:
				if t, err := httpretryafter.Parse(e.response.Header.Get("Retry-After")); err == nil {
					return time.Until(t)
				}
			case SomeOtherError:
				return e.retryAfter
			}

			// default to backoffdelay when server does not supply a retry-after response header
			return retry.BackOffDelay(n, err, config)
		}),
	)

	return err
}
