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
package data

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

const (
	containerNetworkReceiveBytesTotal  = "container_network_receive_bytes_total"
	containerNetworkTransmitBytesTotal = "container_network_transmit_bytes_total"
)

func getMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`container_memory_usage_bytes{image!="", container!="POD", container!=""%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getMemoryRequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_requests{container!="POD", container!="", unit="byte", resource="memory"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPURequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_requests{container!="POD", container!="", unit="core", resource="cpu"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getMemoryLimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_limits{container!="POD", container!="", unit="byte", resource="memory"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPULimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_limits{container!="POD", container!="", unit="core", resource="cpu"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPU(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`rate(container_cpu_usage_seconds_total{image!="", container!="POD", container!=""%s}[2m])`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getNetworkReceiveBytesFor30s(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, minutes int, clusterName string) (model.Matrix, error) {
	return get30sIncreaseMetric(ctx, api, r, containerNetworkReceiveBytesTotal, minutes, clusterName)
}

func getNetworkTransmitBytesFor30s(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, minutes int, clusterName string) (model.Matrix, error) {
	return get30sIncreaseMetric(ctx, api, r, containerNetworkTransmitBytesTotal, minutes, clusterName)
}

func get30sIncreaseMetric(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, metric string, minutes int, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`increase(%s{interface="eth0"%s}[%dm])`, metric, clusterFilter, minutes)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}

	//adjusting values to 30s
	var adjusted model.Matrix
	for _, r := range values {
		matrix := &model.SampleStream{
			Metric: r.Metric,
			Values: []model.SamplePair{},
		}
		for _, v := range r.Values {
			newValue := v
			newValue.Value = model.SampleValue((float64(newValue.Value)) / float64(2*minutes)) // 30s adjustment /2 /minutes
			matrix.Values = append(matrix.Values, newValue)
		}
		adjusted = append(adjusted, matrix)
	}
	logrus.Debugf("returning %s bytes values: %v", metric, spew.Sprintf("%v", adjusted))
	return adjusted, nil
}

func getNodesIdleMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`{cluster="%s"}`, clusterName)
	}
	query := fmt.Sprintf(`100 * ((SUM(avg_over_time(node_memory_MemAvailable_bytes%s[60m])) / SUM(avg_over_time(node_memory_MemTotal_bytes%s[60m]))))`, clusterFilter, clusterFilter)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	return values, nil
}

func getNodesIdleCPU(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`avg(rate(node_cpu_seconds_total{mode="idle", mode!="iowait", mode!="steal"%s}[5m])) * 100`, clusterFilter)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	return values, nil
}

func getNodesOverheadCPU(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	clusterCompleteFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
		clusterCompleteFilter = fmt.Sprintf(`{cluster="%s"}`, clusterName)
	}
	query := fmt.Sprintf(`
		100 * (
			(1 - avg(rate(node_cpu_seconds_total{mode="idle", mode!="iowait", mode!="steal"%s}[5m]))) * SUM(machine_cpu_cores%s) 
			- SUM(rate(container_cpu_usage_seconds_total{image!="", container!="POD", container!=""%s}[10m]))
			) 
		/ SUM(machine_cpu_cores%s)`,
		clusterFilter, clusterCompleteFilter, clusterFilter, clusterCompleteFilter)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	return values, nil
}

func getNodesOverheadMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	clusterCompleteFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
		clusterCompleteFilter = fmt.Sprintf(`{cluster="%s"}`, clusterName)
	}
	query := fmt.Sprintf(`
		100 * 
		(
			(1 - (SUM(avg_over_time(node_memory_MemAvailable_bytes%s[10m])) / SUM(avg_over_time(node_memory_MemTotal_bytes%s[10m])))) * SUM(machine_memory_bytes%s)
			- SUM(container_memory_usage_bytes{image!="", container!="POD", container!=""%s})
		)
		/ SUM(machine_memory_bytes%s)`,
		clusterCompleteFilter, clusterCompleteFilter, clusterCompleteFilter, clusterFilter, clusterCompleteFilter)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	return values, nil
}

func queryPrometheus(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, query string) (model.Matrix, error) {
	if query == "" {
		return model.Matrix{}, fmt.Errorf("query cannot be empty")
	}
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), nil
}

// getGPUUsage fetches GPU utilization from DCGM Exporter.
// DCGM_FI_DEV_GPU_UTIL reports 0-100%, we normalize to 0-1 per GPU.
// This is a pod-level metric (no container label from DCGM).
func getGPUUsage(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	// DCGM exports GPU metrics with namespace and pod labels when running in Kubernetes
	// We average across GPUs per pod and normalize from 0-100 to 0-1
	query := fmt.Sprintf(`avg by (namespace, pod) (DCGM_FI_DEV_GPU_UTIL{namespace!="", pod!=""%s}) / 100`, clusterFilter)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	return values, nil
}

// getGPURequests fetches nvidia.com/gpu resource requests from kube-state-metrics.
func getGPURequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_requests{container!="POD", container!="", resource="nvidia_com_gpu"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

// getGPULimits fetches nvidia.com/gpu resource limits from kube-state-metrics.
func getGPULimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_limits{container!="POD", container!="", resource="nvidia_com_gpu"%s}`, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}
