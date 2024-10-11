package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
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
	NamespaceBlocklist        []string
	NamespaceAllowlist        []string
}

func LoadFromEnvironment() (*config, error) {
	maxConcurrentScans := MAX_CONCURRENT_SCANS
	concurrencyStr := os.Getenv("MAX_CONCURRENT_SCANS")
	if concurrencyStr != "" {
		var err error
		maxConcurrentScans, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			return nil, err
		}
	}

	numberToScan := NUMBER_TO_SCAN
	numberToScanStr := os.Getenv("MAX_SCANS")
	if numberToScanStr != "" {
		var err error
		numberToScan, err = strconv.Atoi(numberToScanStr)
		if err != nil {
			return nil, err
		}
	}

	var extraFlags string
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

	var offline bool
	if os.Getenv("OFFLINE") != "" {
		offline = true
	}

	var serviceAccountAnnotations map[string]string
	serviceAccountAnnotationsStr := os.Getenv("SERVICE_ACCOUNT_ANNOTATIONS")
	if len(serviceAccountAnnotationsStr) > 0 {
		// format is JSON with {string:string} '{"iam.gke.io/gcp-service-account":"my-gsa@my-project.iam.gserviceaccount.com","another-key":"another-value"}'
		err := json.Unmarshal([]byte(serviceAccountAnnotationsStr), &serviceAccountAnnotations)
		if err != nil {
			return nil, err
		}
	}

	var namespaceBlocklist, namespaceAllowlist []string
	if os.Getenv("NAMESPACE_BLACKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLACKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_BLOCKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLOCKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_ALLOWLIST") != "" {
		namespaceAllowlist = strings.Split(os.Getenv("NAMESPACE_ALLOWLIST"), ",")
	}

	return &config{
		Offline:                   offline,
		MaxConcurrentScans:        maxConcurrentScans,
		NumberToScan:              numberToScan,
		ExtraFlags:                extraFlags,
		ServiceAccountAnnotations: serviceAccountAnnotations,
		NamespaceBlocklist:        namespaceBlocklist,
		NamespaceAllowlist:        namespaceAllowlist,
	}, nil
}
