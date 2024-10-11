package config

import (
	"encoding/json"
	"os"
	"strconv"
)

const (
	MAX_CONCURRENT_SCANS = 5
	NUMBER_TO_SCAN       = 10
)

type config struct {
	Offline                   bool
	MaxConcurrentScans        int
	NumberToScan              int
	ExtraFlags                string
	ServiceAccountAnnotations map[string]string
}

func LoadFromEnvironment() (*config, error) {
	maxConcurrentScans := MAX_CONCURRENT_SCANS
	numberToScan := NUMBER_TO_SCAN
	var extraFlags string
	var serviceAccountAnnotations map[string]string
	var offline bool

	concurrencyStr := os.Getenv("MAX_CONCURRENT_SCANS")
	if concurrencyStr != "" {
		var err error
		maxConcurrentScans, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			return nil, err
		}
	}

	numberToScanStr := os.Getenv("MAX_SCANS")
	if numberToScanStr != "" {
		var err error
		numberToScan, err = strconv.Atoi(numberToScanStr)
		if err != nil {
			return nil, err
		}
	}

	ignoreUnfixedStr := os.Getenv("IGNORE_UNFIXED")
	if ignoreUnfixedStr != "" {
		ignoreUnfixedBool, err := strconv.ParseBool(ignoreUnfixedStr)
		if err != nil {
			return nil, err
		}
		if ignoreUnfixedBool {
			extraFlags = "--ignore-unfixed"
		}
	}

	if os.Getenv("OFFLINE") != "" {
		offline = true
	}

	serviceAccountAnnotationsStr := os.Getenv("SERVICE_ACCOUNT_ANNOTATIONS")
	if len(serviceAccountAnnotationsStr) > 0 {
		// format is JSON with {string:string} "{"iam.gke.io/gcp-service-account":"my-gsa@my-project.iam.gserviceaccount.com","another-key":"another-value"}"
		err := json.Unmarshal([]byte(serviceAccountAnnotationsStr), &serviceAccountAnnotations)
		if err != nil {
			return nil, err
		}
	}

	return &config{
		Offline:                   offline,
		MaxConcurrentScans:        maxConcurrentScans,
		NumberToScan:              numberToScan,
		ExtraFlags:                extraFlags,
		ServiceAccountAnnotations: serviceAccountAnnotations,
	}, nil
}
