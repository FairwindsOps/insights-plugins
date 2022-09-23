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
	"regexp"
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
		// 5 digit alphanumeric (pod) or strictly numeric segments (cronjob -> job, statefulset). or 9 digit alphanumberic (deployment -> rs)
		matched, err := regexp.Match(fmt.Sprintf("^%s-([a-z0-9]{5}|[a-z0-9]{9}|[0-9]*)(-[a-z0-9]{5})?$", workload.TopController.GetName()), []byte(podName))
		if err != nil {
			logrus.Error(err)
			return
		}
		if matched {
			// Weak match for a pod. Don't return yet in case there's a better match.
			name = workload.TopController.GetName()
			kind = workload.TopController.GetKind()
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
	requestArray := make([]CombinedRequest, 0, len(combinedRequests))
	workloads, err := controller.GetAllTopControllers(ctx, dynamicClient, restMapper, "")
	if err != nil {
		return nil, err
	}
	workloadMap := make(map[string]controller.Workload)
	for _, workload := range workloads {
		for _, pod := range workload.Pods {
			workloadMap[fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())] = workload
		}
	}
	for _, val := range combinedRequests {
		workload, ok := workloadMap[fmt.Sprintf("%s/%s", val.ControllerNamespace, val.PodName)]
		if !ok {
			val.ControllerName, val.ControllerKind = getController(workloads, val.PodName, val.ControllerNamespace)
		} else {
			val.ControllerName = workload.TopController.GetName()
			val.ControllerKind = workload.TopController.GetKind()
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
