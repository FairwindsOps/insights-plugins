package data

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

// Owner is the information about a pod that a set of metrics belongs to.
type Owner struct {
	Container      string
	Pod            string
	Namespace      string
	ControllerName string
	ControllerKind string
}

// Statistics is an aggregation of the metrics by pod/container
type Statistics struct {
	Owner
	Timestamp time.Time
	Stats     map[string]float64
}

// CombinedRequest is the cpu/memory and requests for a given pod/container
type CombinedRequest struct {
	Owner
	cpu           []model.SamplePair
	memory        []model.SamplePair
	memoryRequest model.SampleValue
	cpuRequest    model.SampleValue
}

// GetClient returns a Prometheus API client for a given address
func GetClient(address string) (prometheusV1.API, error) {
	config := api.Config{
		Address:      address,
		RoundTripper: api.DefaultRoundTripper,
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return prometheusV1.NewAPI(client), nil
}

func getRange() prometheusV1.Range {
	return prometheusV1.Range{
		Start: time.Now().Truncate(time.Hour).Add(-2 * time.Hour),
		End:   time.Now().Truncate(time.Hour),
		Step:  2 * time.Second,
	}
}

// CalculateStatistics finds the max/min/avg for a set of data points by hour
func CalculateStatistics(values []CombinedRequest) []Statistics {
	stats := make([]Statistics, 0)
	type memCPU struct {
		memoryArray []float64
		cpuArray    []float64
	}
	for _, value := range values {
		valMap := map[time.Time]memCPU{}
		for _, mem := range value.memory {
			timestamp := time.Unix(int64(mem.Timestamp), 0).Truncate(time.Hour)
			memValues := valMap[timestamp]
			memValues.memoryArray = append(memValues.memoryArray, float64(mem.Value))
			valMap[timestamp] = memValues
		}
		for _, cpu := range value.cpu {
			timestamp := time.Unix(int64(cpu.Timestamp), 0).Truncate(time.Hour)
			cpuValues := valMap[timestamp]
			cpuValues.cpuArray = append(cpuValues.cpuArray, float64(cpu.Value))
			valMap[timestamp] = cpuValues
		}
		for ts, values := range valMap {
			stat := Statistics{}
			stat.Owner = value.Owner
			max, min, avg, floorAverage, stdDev := calculateMaxMin(values.memoryArray, float64(value.memoryRequest))
			stat.Stats["max-memory"] = max
			stat.Stats["min-memory"] = min
			stat.Stats["avg-memory"] = avg
			stat.Stats["floorAverage-memory"] = floorAverage
			stat.Stats["standardDeviation-memory"] = stdDev
			max, min, avg, floorAverage, stdDev = calculateMaxMin(values.cpuArray, float64(value.cpuRequest))
			stat.Stats["max-cpu"] = max
			stat.Stats["min-cpu"] = min
			stat.Stats["avg-cpu"] = avg
			stat.Stats["floorAverage-cpu"] = floorAverage
			stat.Stats["standardDeviation-cpu"] = stdDev
			stat.Timestamp = ts
			stats = append(stats, stat)
		}
	}

	return stats
}

func getController(workloads []controller.Workload, podName, namespace string) (name, kind string) {
	for _, workload := range workloads {
		if workload.TopController.GetNamespace() != namespace {
			continue
		}
		for _, pod := range workload.Pods {
			if pod.GetName() == podName {
				// Exact match for a pod, go ahead and return
				name = workload.TopController.GetName()
				kind = workload.TopController.GetKind()
				return
			}
		}
		// TODO find what's left and make sure it looks like an automatic prefix
		// 5 digit alpha or strictly numeric segments.
		if strings.HasPrefix(podName, fmt.Sprintf("%s-", workload.TopController.GetName())) {
			// Weak match for a pod. Don't return yet in case there's a better match.
			name = workload.TopController.GetName()
			kind = workload.TopController.GetKind()
		}
	}
	return
}

func calculateMaxMin(values []float64, request float64) (max, min, avg, floorAverage, stdDev float64) {
	if len(values) == 0 {
		return
	}
	min = values[0]
	for _, val := range values {
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
		avg += val
		if request > val {
			floorAverage += float64(request)
		} else {
			floorAverage += val
		}
	}
	length := float64(len(values))
	avg = avg / length
	floorAverage = floorAverage / length
	for _, val := range values {
		floatVal := val
		stdDev += math.Pow(floatVal-avg, 2)
	}

	// Avoid a divide by zero for the standard deviation
	if length < 2 {
		length = 2
	}
	stdDev = math.Sqrt(stdDev / (length - 1))
	return
}

// GetMetrics returns the memory/cpu and requests for each container running in the cluster.
func GetMetrics(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, api prometheusV1.API) ([]CombinedRequest, error) {
	r := getRange()
	memory, err := getMemory(ctx, api, r)
	if err != nil {
		return nil, err
	}
	cpu, err := getCPU(ctx, api, r)
	if err != nil {
		return nil, err
	}
	memoryRequest, err := getMemoryRequests(ctx, api, r)
	if err != nil {
		return nil, err
	}
	cpuRequest, err := getCPURequests(ctx, api, r)
	if err != nil {
		return nil, err
	}
	combinedRequests := make(map[string]CombinedRequest)
	for _, cpuVal := range cpu {
		key := getKey(cpuVal)
		request := combinedRequests[key]
		request.cpu = cpuVal.Values
		request.Owner = getOwner(cpuVal)
		combinedRequests[key] = request
	}
	for _, memVal := range memory {
		key := getKey(memVal)
		request := combinedRequests[key]
		request.memory = memVal.Values
		request.Owner = getOwner(memVal)
		combinedRequests[key] = request
	}
	for _, cpuVal := range cpuRequest {
		key := getKey(cpuVal)
		request := combinedRequests[key]
		request.Owner = getOwner(cpuVal)
		request.cpuRequest = cpuVal.Values[0].Value
		combinedRequests[key] = request
	}
	for _, memVal := range memoryRequest {
		key := getKey(memVal)
		request := combinedRequests[key]
		request.Owner = getOwner(memVal)
		request.memoryRequest = memVal.Values[0].Value
		combinedRequests[key] = request
	}
	requestArray := make([]CombinedRequest, 0, len(combinedRequests))
	workloads, err := controller.GetAllTopControllers(ctx, dynamicClient, restMapper, "")
	if err != nil {
		return nil, err
	}
	for _, val := range combinedRequests {
		val.ControllerName, val.ControllerKind = getController(workloads, val.Pod, val.Namespace)
		requestArray = append(requestArray, val)
	}
	// TODO send results to Insights
	return requestArray, nil
}

func getKey(sample *model.SampleStream) string {
	return fmt.Sprintf("%s/%s/%s", sample.Metric["namespace"], sample.Metric["pod"], sample.Metric["container"])
}

func getOwner(sample *model.SampleStream) Owner {
	return Owner{
		Namespace: string(sample.Metric["namespace"]),
		Pod:       string(sample.Metric["pod"]),
		Container: string(sample.Metric["container"]),
	}
}
func getMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `container_memory_usage_bytes{image!="", id=~"/kubepods.*", container!="POD", container!=""}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	return values.(model.Matrix), err
}

func getMemoryRequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_requests_memory_bytes{image!="", id=~"/kubepods.*", container!="POD", container!=""}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	return values.(model.Matrix), err
}

func getCPURequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_requests_cpu_cores{image!="", id=~"/kubepods.*", container!="POD", container!=""}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	return values.(model.Matrix), err
}

func getCPU(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `rate(container_cpu_usage_seconds_total{image!="", id=~"/kubepods.*", container!="POD", container!=""}[5m])`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	return values.(model.Matrix), err
}
