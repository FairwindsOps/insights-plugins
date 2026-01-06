// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fairwindsops/insights-plugins/plugins/prometheus/pkg/data"
)

const outputFile = "/output/prometheus-metrics.json"
const outputTempFile = "/output/prometheus-metrics-tmp.json"
const monitoringReadScope = "https://www.googleapis.com/auth/monitoring.read"
const monitoringGoogleApis = "monitoring.googleapis.com"

func main() {
	setLogLevel()
	address := os.Getenv("PROMETHEUS_ADDRESS")
	if address == "" {
		panic("prometheus-metrics.address must be set")
	}
	clusterName := os.Getenv("CLUSTER_NAME")
	skipNonZeroMetricsValidation := strings.ToLower(os.Getenv("SKIP_NON_ZERO_METRICS_CHECK")) == "true"
	skipKSMNonZeroMetricsValidation := strings.ToLower(os.Getenv("SKIP_KSM_NON_ZERO_METRICS_CHECK")) == "" || strings.ToLower(os.Getenv("SKIP_KSM_NON_ZERO_METRICS_CHECK")) == "true"

	// Build client options based on environment configuration
	var clientOpts []data.ClientOption

	// Handle bearer token authentication (Google Cloud Monitoring or explicit token)
	accessToken := os.Getenv("PROMETHEUS_BEARER_TOKEN")
	if strings.Contains(address, monitoringGoogleApis) {
		tokenSource, err := google.DefaultTokenSource(context.Background(), monitoringReadScope)
		if err != nil {
			panic(err)
		}
		token, err := tokenSource.Token()
		if err != nil {
			panic(err)
		}
		accessToken = token.AccessToken
	}
	if accessToken != "" {
		clientOpts = append(clientOpts, data.WithBearerToken(accessToken))
	}

	// Handle multi-tenant Prometheus backends (e.g., Grafana Mimir)
	tenantID := os.Getenv("PROMETHEUS_TENANT_ID")
	if tenantID != "" {
		logrus.Infof("Using tenant ID: %s", tenantID)
		clientOpts = append(clientOpts, data.WithTenantID(tenantID))
	}

	logrus.Infof("Getting metrics from Prometheus at %s", address)
	client, err := data.GetClientWithOptions(address, clientOpts...)
	if err != nil {
		panic(err)
	}

	dynamic, restMapper, err := getKubeClient()
	if err != nil {
		panic(err)
	}
	res, err := data.GetMetrics(context.Background(), dynamic, restMapper, client, clusterName, skipNonZeroMetricsValidation, skipKSMNonZeroMetricsValidation)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Got %d metrics", len(res))
	stats := data.CalculateStatistics(res)

	nodesMetrics, err := data.GetNodesMetrics(context.Background(), dynamic, restMapper, client, clusterName)
	if err != nil {
		panic(err)
	}
	data, err := json.Marshal(map[string]interface{}{
		"Values": stats,
		"Nodes":  nodesMetrics,
	})
	if err != nil {
		panic(err)
	}
	logrus.Infof("Aggregated to %d statistics", len(stats))
	err = os.WriteFile(outputTempFile, data, 0644)
	if err != nil {
		panic(err)
	}
	err = os.Rename(outputTempFile, outputFile)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Done!")
}

func setLogLevel() {
	if os.Getenv("LOGRUS_LEVEL") != "" {
		lvl, err := logrus.ParseLevel(os.Getenv("LOGRUS_LEVEL"))
		if err != nil {
			panic(fmt.Errorf("invalid log level %q (should be one of trace, debug, info, warning, error, fatal, panic), error: %v", os.Getenv("LOGRUS_LEVEL"), err))
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func getKubeClient() (dynamic.Interface, meta.RESTMapper, error) {
	var restMapper meta.RESTMapper
	var dynamicClient dynamic.Interface
	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		logrus.Errorf("Error fetching KubeConfig: %v", configError)
		return dynamicClient, restMapper, configError
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Kubernetes client: %v", err)
		return dynamicClient, restMapper, err
	}

	dynamicClient, err = dynamic.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Dynamic client: %v", err)
		return dynamicClient, restMapper, err
	}

	resources, err := restmapper.GetAPIGroupResources(api.Discovery())
	if err != nil {
		logrus.Errorf("Error getting API Group resources: %v", err)
		return dynamicClient, restMapper, err
	}
	restMapper = restmapper.NewDiscoveryRESTMapper(resources)
	return dynamicClient, restMapper, nil
}
