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
	"maps"
	"strings"

	"github.com/davecgh/go-spew/spew"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

const (
	containerNetworkReceiveBytesTotal  = "container_network_receive_bytes_total"
	containerNetworkTransmitBytesTotal = "container_network_transmit_bytes_total"
)

const gkeSystemMetricsPrefix = "kubernetes_io:container_"

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

func validateClusterNameForPromQL(clusterName string) error {
	if strings.ContainsAny(clusterName, "\"\\\n\r") {
		return fmt.Errorf("cluster name contains unsafe character for PromQL")
	}
	return nil
}

// queryGKEContainerMetric runs a PromQL query for a GKE system metric (k8s_container
// resource), with cluster filter. Returns matrix with labels that may use
// namespace_name/pod_name/container_name; call normalizeGKEContainerMatrix before use.
// See: https://cloud.google.com/monitoring/api/resources#tag_k8s_container (cluster_name is a resource label).
func queryGKEContainerMetric(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string, metricName string, sumByContainer bool) (model.Matrix, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name required for GKE system metrics query")
	}
	if err := validateClusterNameForPromQL(clusterName); err != nil {
		return nil, err
	}
	// GKE k8s_container resource uses cluster_name; monitored_resource disambiguates.
	selector := fmt.Sprintf(`%s%s{monitored_resource="k8s_container", cluster_name="%s"}`, gkeSystemMetricsPrefix, metricName, clusterName)
	query := selector
	if sumByContainer {
		// Multiple series per container (e.g. accelerator per resource_name); sum to one per container.
		query = fmt.Sprintf(`sum by (namespace_name, pod_name, container_name) (%s)`, selector)
	}
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	matrix, ok := values.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected Matrix from QueryRange, got %T", values)
	}
	return matrix, nil
}

// normalizeGKEContainerMatrix copies each series and normalizes labels to namespace, pod, container
// so getKey() and getOwner() work. GKE k8s_container uses namespace_name, pod_name, container_name
// (or sometimes namespace, pod, container); we unify to the short form.
func normalizeGKEContainerMatrix(m model.Matrix) model.Matrix {
	if len(m) == 0 {
		return m
	}
	out := make(model.Matrix, 0, len(m))
	for _, stream := range m {
		norm := make(model.Metric)
		maps.Copy(norm, stream.Metric)
		// Prefer _name suffix (GKE resource labels), fall back to short form.
		if v, ok := stream.Metric["namespace_name"]; ok {
			norm["namespace"] = v
		} else if v, ok := stream.Metric["namespace"]; ok {
			norm["namespace"] = v
		}
		if v, ok := stream.Metric["pod_name"]; ok {
			norm["pod"] = v
		} else if v, ok := stream.Metric["pod"]; ok {
			norm["pod"] = v
		}
		if v, ok := stream.Metric["container_name"]; ok {
			norm["container"] = v
		} else if v, ok := stream.Metric["container"]; ok {
			norm["container"] = v
		}
		out = append(out, &model.SampleStream{Metric: norm, Values: stream.Values})
	}
	return out
}

func getMemoryRequestsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "memory_request_bytes", false)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

func getMemoryLimitsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "memory_limit_bytes", false)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

func getCPURequestsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "cpu_request_cores", false)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

func getCPULimitsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "cpu_limit_cores", false)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

func getGPURequestsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "accelerator_request", true)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

func getGPULimitsGKE(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	// GKE system metrics expose container/accelerator/request but not a separate "limit".
	// Use the same request metric as best-effort limit proxy when KSM has no limits.
	m, err := queryGKEContainerMetric(ctx, api, r, clusterName, "accelerator_request", true)
	if err != nil {
		return nil, err
	}
	return normalizeGKEContainerMatrix(m), nil
}

// =============================================================================
// GPU METRICS - Multi-vendor support
// =============================================================================
// Supported GPU/accelerator types:
// - NVIDIA: nvidia.com/gpu, nvidia.com/gpu.shared (time-sliced)
// - AMD: amd.com/gpu
// - Intel: intel.com/gpu
// - Habana Gaudi: habana.ai/gaudi
// - Google TPU: google.com/tpu
// - AWS vGPU: k8s.amazonaws.com/vgpu

// gpuResourcePattern matches all supported GPU/accelerator resource names in kube-state-metrics.
// Note: kube-state-metrics converts "/" and "." to "_" in resource names.
const gpuResourcePattern = `nvidia_com_gpu|nvidia_com_gpu_shared|k8s_amazonaws_com_vgpu|amd_com_gpu|intel_com_gpu|habana_ai_gaudi|google_com_tpu`

// gpuUtilizationQuery contains vendor-specific GPU utilization queries.
// Each vendor has a different exporter with different metric names.
// Queries normalize utilization to 0-1 range per GPU.
type gpuUtilizationQuery struct {
	name  string // Vendor name for logging
	query string // PromQL query template (use %s for cluster filter)
}

// gpuUtilizationQueries lists all vendor-specific GPU utilization queries.
// Missing exporters will return empty results (not errors).
var gpuUtilizationQueries = []gpuUtilizationQuery{
	// NVIDIA DCGM Exporter - reports 0-100%
	{"nvidia", `avg by (namespace, pod) (DCGM_FI_DEV_GPU_UTIL{namespace!="", pod!=""%s}) / 100`},
	// AMD Device Metrics Exporter (ROCm) - reports 0-100%
	// GPU_GFX_ACTIVITY measures graphics engine usage percentage
	{"amd", `avg by (namespace, pod) (GPU_GFX_ACTIVITY{namespace!="", pod!=""%s}) / 100`},
	// Intel GPU Plugin - reports 0-1
	{"intel", `avg by (namespace, pod) (intel_gpu_engine_render_active{namespace!="", pod!=""%s})`},
	// Habana HL-SMI Exporter - reports 0-100%
	{"habana", `avg by (namespace, pod) (hl_utilization{namespace!="", pod!=""%s}) / 100`},
}

// getGPUUsage fetches GPU utilization from all vendor-specific exporters.
// It queries each exporter and merges results. Missing exporters are ignored.
// Returns pod-level utilization (no container label from GPU exporters).
func getGPUUsage(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}

	var allResults model.Matrix
	for _, q := range gpuUtilizationQueries {
		query := fmt.Sprintf(q.query, clusterFilter)
		values, err := queryPrometheus(ctx, api, r, query)
		if err != nil {
			// Exporter not installed for this vendor - this is expected
			logrus.Debugf("%s GPU utilization not available: %v", q.name, err)
			continue
		}
		if len(values) > 0 {
			logrus.Debugf("Found %d metrics for %s GPU utilization", len(values), q.name)
			allResults = append(allResults, values...)
		}
	}

	return allResults, nil
}

// getGPURequests fetches GPU resource requests from kube-state-metrics for ALL vendors.
// Uses regex matching to support all GPU resource types.
func getGPURequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_requests{container!="POD", container!="", resource=~"%s"%s}`, gpuResourcePattern, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

// getGPULimits fetches GPU resource limits from kube-state-metrics for ALL vendors.
// Uses regex matching to support all GPU resource types.
func getGPULimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, clusterName string) (model.Matrix, error) {
	clusterFilter := ""
	if clusterName != "" {
		clusterFilter = fmt.Sprintf(`, cluster="%s"`, clusterName)
	}
	query := fmt.Sprintf(`kube_pod_container_resource_limits{container!="POD", container!="", resource=~"%s"%s}`, gpuResourcePattern, clusterFilter)
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}
