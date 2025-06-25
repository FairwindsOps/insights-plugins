package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

const (
	MAX_CONCURRENT_SCANS = 5
	MAX_IMAGES_TO_SCAN   = 10
)

type config struct {
	Offline            bool
	MaxConcurrentScans int
	MaxImagesToScan    int
	ExtraFlags         string
	NamespaceBlocklist []string
	NamespaceAllowlist []string
	HasGKESAAnnotation bool
	ImagesToScan       []string
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

	maxImagesToScan := MAX_IMAGES_TO_SCAN
	maxScansEnvVar := os.Getenv("MAX_SCANS")
	if maxScansEnvVar != "" {
		var err error
		maxImagesToScan, err = strconv.Atoi(maxScansEnvVar)
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

	var namespaceBlocklist, namespaceAllowlist, imagesToScan []string
	if os.Getenv("NAMESPACE_BLACKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLACKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_BLOCKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLOCKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_ALLOWLIST") != "" {
		namespaceAllowlist = strings.Split(os.Getenv("NAMESPACE_ALLOWLIST"), ",")
	}
	if os.Getenv("IMAGES_TO_SCAN") != "" {
		imagesToScan = strings.Split(os.Getenv("IMAGES_TO_SCAN"), ",")
	}

	hasGKESAAnnotation := false
	if _, ok := serviceAccountAnnotations["iam.gke.io/gcp-service-account"]; ok {
		hasGKESAAnnotation = true
	}

	return &config{
		Offline:            offline,
		MaxConcurrentScans: maxConcurrentScans,
		MaxImagesToScan:    maxImagesToScan,
		ExtraFlags:         extraFlags,
		NamespaceBlocklist: namespaceBlocklist,
		NamespaceAllowlist: namespaceAllowlist,
		HasGKESAAnnotation: hasGKESAAnnotation,
		ImagesToScan:       imagesToScan,
	}, nil
}
