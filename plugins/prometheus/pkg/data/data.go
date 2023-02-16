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

const urlFormat = "%s/v0/organizations/%s/clusters/%s/data/metrics"

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

// TODO make configurable
var timestep time.Duration = time.Minute * 5

func getRange() prometheusV1.Range {
	return prometheusV1.Range{
		Start: time.Now().Truncate(timestep).Add(timestep * -3),
		End:   time.Now().Truncate(timestep),
		Step:  30 * time.Second,
	}
}

func getController(workloads []controller.Workload, podName, namespace string) (name, kind string) {
	name = podName
	kind = "Pod"
	prefixMatchLength := 0
	for _, workload := range workloads {
		if workload.TopController.GetNamespace() != namespace {
			continue
		}

		workloadName := workload.TopController.GetName()
		workloadKind := workload.TopController.GetKind()
		for _, pod := range workload.Pods {
			if pod.GetName() == podName {
				// Exact match for a pod, go ahead and return
				name = workloadName
				kind = workloadKind
				return
			}
		}

		isMatch := strings.HasPrefix(podName, workloadName)
		if !isMatch {
			continue
		}

		isBetterMatch := len(workloadName) > prefixMatchLength
		if isBetterMatch {
			prefixMatchLength = len(workloadName)
			name = workloadName
			kind = workloadKind
		}
	}
	return
}

// GetMetrics returns the memory/cpu and requests for each container running in the cluster.
func GetMetrics(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, api prometheusV1.API) ([]CombinedRequest, error) {
	r := getRange()
	memory, err := getMemory(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for memory", len(memory))

	cpu, err := getCPU(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for cpu", len(cpu))

	memoryRequest, err := getMemoryRequests(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for memoryRequests", len(memoryRequest))

	cpuRequest, err := getCPURequests(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for cpuRequests", len(cpuRequest))

	memoryLimits, err := getMemoryLimits(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for memoryLimits", len(memoryLimits))

	cpuLimits, err := getCPULimits(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for cpuLimits", len(cpuLimits))

	networkTransmit, err := getNetworkTransmitBytesIncludingBaseline(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for network transmit bytes", len(networkTransmit))

	networkReceive, err := getNetworkReceiveBytesIncludingBaseline(ctx, api, r)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d metrics for network receive bytes", len(networkReceive))

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
	for _, cpuVal := range cpuLimits {
		key := getKey(cpuVal)
		request := combinedRequests[key]
		request.Owner = getOwner(cpuVal)
		request.cpuLimit = cpuVal.Values[0].Value
		combinedRequests[key] = request
	}
	for _, memVal := range memoryLimits {
		key := getKey(memVal)
		request := combinedRequests[key]
		request.Owner = getOwner(memVal)
		request.memoryLimit = memVal.Values[0].Value
		combinedRequests[key] = request
	}

	// Information about pods is required to lookup the number of containers
	// each pod has, to manipulate per-pod network metrics to appear to be
	// per-container.
	client := controller.Client{
		Context:    ctx,
		Dynamic:    dynamicClient,
		RESTMapper: restMapper,
	}
	// workloads, err := client.GetAllTopControllersSummary("")
	workloads, err := client.GetAllTopControllersWithPods("")
	if err != nil {
		return nil, err
	}
	workloadMap := make(map[string]*controller.Workload)
	for idx, workload := range workloads {
		for _, pod := range workload.Pods {
			workloadMap[fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())] = &workloads[idx]
		}
	}

	for _, networkVal := range networkTransmit {
		deltaValues, err := cumulitiveValuesToDeltaValues(networkVal.Values, r)
		if err != nil {
			logrus.Warnf("while converting network transmit values from cumulitive to deltas: %v", err)
			continue
		}
		networkVal.Values = deltaValues
	}

	networkTransmit = adjustMetricsForMultiContainerPods(networkTransmit, workloadMap)
	for _, networkVal := range networkTransmit {
		key := getKey(networkVal)
		request := combinedRequests[key]
		request.networkTransmit = networkVal.Values
		request.Owner = getOwner(networkVal)
		combinedRequests[key] = request
	}

	for _, networkVal := range networkReceive {
		deltaValues, err := cumulitiveValuesToDeltaValues(networkVal.Values, r)
		if err != nil {
			logrus.Warnf("while converting network receive values from cumulitive to deltas: %v", err)
			continue
		}
		networkVal.Values = deltaValues
	}

	networkReceive = adjustMetricsForMultiContainerPods(networkReceive, workloadMap)
	for _, networkVal := range networkReceive {
		key := getKey(networkVal)
		request := combinedRequests[key]
		request.networkReceive = networkVal.Values
		request.Owner = getOwner(networkVal)
		combinedRequests[key] = request
	}

	requestArray := make([]CombinedRequest, 0, len(combinedRequests))

	for _, val := range combinedRequests {
		if workload, ok := workloadMap[fmt.Sprintf("%s/%s", val.ControllerNamespace, val.PodName)]; ok {
			val.ControllerName = workload.TopController.GetName()
			val.ControllerKind = workload.TopController.GetKind()
		} else {
			val.ControllerName, val.ControllerKind = getController(workloads, val.PodName, val.ControllerNamespace)
			logrus.Infof("Could not find owner for pod %s in namespace %s, using %s/%s", val.PodName, val.ControllerNamespace, val.ControllerKind, val.ControllerName)
		}
		requestArray = append(requestArray, val)
	}
	return requestArray, nil
}

func getKey(sample *model.SampleStream) string {
	return fmt.Sprintf("%s/%s/%s", sample.Metric["namespace"], sample.Metric["pod"], sample.Metric["container"])
}

func getOwner(sample *model.SampleStream) Owner {
	return Owner{
		ControllerNamespace: string(sample.Metric["namespace"]),
		PodName:             string(sample.Metric["pod"]),
		Container:           string(sample.Metric["container"]),
	}
}

// adjustMetricsForMultiContainerPods splits values for any metrics
// whos pod has more than one container. The original values
// will be divided by the number of containers, with any remainder assigned to
// the first container.
//
// For example, a pod with 3 containers, and a
// metric value of 10, will be converted to 3 metrics. The metric for the
// first container will have value 4, and the second and third metrics will
// each have a value of 3.
// A metric value of 1.5, will be converted to 3 metrics. The metric for the
// first container will have value 1.5, and the second and third metrics will
// each have a value of 0.
//
// The supplied map of Kubernetes workloads is used to determine the number of
// containers and their names, for pods referenced in each metric. The map is
// keyed on a string `{namespace}/{podName}`.
func adjustMetricsForMultiContainerPods(metrics model.Matrix, workloadMap map[string]*controller.Workload) model.Matrix {
	newMetrics := make(model.Matrix, 0)
	for _, sample := range metrics {
		podKey := fmt.Sprintf("%s/%s", sample.Metric["namespace"], sample.Metric["pod"])
		workload, foundPod := workloadMap[podKey]
		if !foundPod {
			logrus.Warnf("cannot split metrics across the pod's containers, no workload was found for %q", podKey)
			continue
		}
		podContainers := workload.PodSpec.Containers
		numContainers := len(podContainers)
		if numContainers == 1 {
			logrus.Debugf("assigning pod-metrics for pod %s to its only container, %s", podKey, podContainers[0].Name)
			sample.Metric["container"] = model.LabelValue(podContainers[0].Name)
			newMetrics = append(newMetrics, sample)
			continue
		} else {
			logrus.Debugf("splitting pod-metric values for pod %s across its %d containers", podKey, len(podContainers))
			sample.Metric["container"] = model.LabelValue(podContainers[0].Name)
			origValues := make([]model.SampleValue, len(sample.Values))
			for i, v := range sample.Values {
				origValues[i] = v.Value
			}
			splitValues := make([]model.SampleValue, len(sample.Values))
			// Divide values by the number of containers, and set metric values for
			// the first container to the equal split + remainder
			for i, v := range origValues {
				vFloat := float64(v)
				remainder := math.Mod(vFloat, float64(numContainers))
				equalSplit := math.Floor(vFloat / float64(numContainers))
				var newValue float64
				var newValueDescr string
				if remainder > 0.0 {
					newValue = equalSplit + remainder
					newValueDescr = "the equal split and remainder"
				} else {
					newValue = equalSplit
					newValueDescr = "the equal split"
				}
				sample.Values[i].Value = model.SampleValue(newValue)
				logrus.Debugf("splitting metric %.1f across %d containers: metric index=%d, remainder=%.1f, equal split=%.1f", vFloat, numContainers, i, remainder, equalSplit)
				logrus.Debugf("setting the new value for the first container (%s) of %s to %s, new value=%.1f, time=%s, metric index=%d", sample.Metric["container"], podKey, newValueDescr, newValue, sample.Values[i].Timestamp, i)
				splitValues[i] = model.SampleValue(equalSplit) // Save the equal splits to populate additional containers' values.
			}
			newMetrics = append(newMetrics, sample) // first container
			// ITerate the rest of the containers and create new metrics for them.
			for containerIndex := 1; containerIndex <= len(podContainers)-1; containerIndex++ {
				newSample := &model.SampleStream{}
				newSample.Metric = make(model.Metric)
				for k, v := range sample.Metric { // Copy key=values from the first container
					newSample.Metric[k] = v
				}
				newSample.Metric["container"] = model.LabelValue(podContainers[containerIndex].Name)
				newSample.Values = make([]model.SamplePair, len(splitValues))
				for i, v := range sample.Values {
					newSample.Values[i].Timestamp = v.Timestamp // keep timestamp from original value
					newSample.Values[i].Value = splitValues[i]  // Use equally split value associated with this index
					logrus.Debugf("setting the new value for container #%d (%s) of %s to %.1f, orig value=%.1f, time=%s, metric index=%d", containerIndex+1, newSample.Metric["container"], podKey, splitValues[i], origValues[i], newSample.Values[i].Timestamp, i)
				}
				newMetrics = append(newMetrics, newSample)
			}
		}
	}
	logrus.Infof("Added %d new metrics while splitting metrics of multi-container pods", len(newMetrics)-len(metrics))
	return newMetrics
}

// cumulitiveValuesToDeltaValues converts the slice of model.SamplePair from
// total; cumulitive values, into delta ones. This expects the slice of
// SamplePair to have at least one value outside of the time range specified by the
// prometheusV1.Range, which will be used as a baseline value to calculate the
// first delta value.
func cumulitiveValuesToDeltaValues(v []model.SamplePair, r prometheusV1.Range) ([]model.SamplePair, error) {
	logrus.Debugf("converting %d values from cumulitive into delta ones, within the time-range %s - %s", len(v), r.Start, r.End)
	indexOfValuesStartingOriginalRange := -1 // sentinal value
	for i := len(v) - 1; i >= 0; i-- {
		logrus.Debugf("index %d is value=%.1f, timestamp=%s", i, v[i].Value, v[i].Timestamp.Time().UTC().String())
		if v[i].Timestamp.Time().After(r.End) { // timestamp after the prometheus range
			logrus.Debugf("ignoring index %d (value %.1f) because its time %s is later than the prometheus range %s", i, v[i].Value, v[i].Timestamp.String(), r.End.String())
			continue
		}
		if v[i].Timestamp.Time().Before(r.Start) { // timestamp before the prometheus range
			logrus.Debugf("index %d is the base value used to calculate subsequent deltas, because it falls outside the prometheus range beginning at %s", i, r.Start.String())
			indexOfValuesStartingOriginalRange = i + 1
			break
		}
		if i >= 1 { // avoid panic if i-1 is < 0
			v[i].Value = v[i].Value - v[i-1].Value
			logrus.Debugf("the new delta value for index %d is %.1f", i, v[i].Value)
		}
	}
	if indexOfValuesStartingOriginalRange == -1 { // never found a base value for all other delta values
		return nil, fmt.Errorf("deltas could not be created from totals because there were no metrics outside of this prometheus range ot establish a baseline: %#v", r)
	}
	v = v[indexOfValuesStartingOriginalRange:]
	logrus.Debugf("the %d values will be shrunk by %d as index %d was the start of values within the prometheus range", len(v), indexOfValuesStartingOriginalRange+1, indexOfValuesStartingOriginalRange)
	return v, nil
}
