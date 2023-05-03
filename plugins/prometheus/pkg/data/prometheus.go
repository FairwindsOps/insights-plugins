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
	"time"

	"github.com/davecgh/go-spew/spew"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

const (
	containerNetworkReceiveBytesTotal  = "container_network_receive_bytes_total"
	containerNetworkTransmitBytesTotal = "container_network_transmit_bytes_total"
)

func getMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `container_memory_usage_bytes{image!="", container!="POD", container!=""}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getMemoryRequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_requests{container!="POD", container!="", unit="byte", resource="memory"}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPURequests(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_requests{container!="POD", container!="", unit="core", resource="cpu"}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getMemoryLimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_limits{container!="POD", container!="", unit="byte", resource="memory"}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPULimits(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `kube_pod_container_resource_limits{container!="POD", container!="", unit="core", resource="cpu"}`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func getCPU(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `rate(container_cpu_usage_seconds_total{image!="", container!="POD", container!=""}[2m])`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}

func get30sNetworkReceiveBytes(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, minutes int) (model.Matrix, error) {
	return get30sIncreaseMetric(ctx, api, r, containerNetworkReceiveBytesTotal, minutes)
}

func get30sNetworkTransmitBytes(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, minutes int) (model.Matrix, error) {
	return get30sIncreaseMetric(ctx, api, r, containerNetworkTransmitBytesTotal, minutes)
}

func get30sIncreaseMetric(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, metric string, minutes int) (model.Matrix, error) {
	query := fmt.Sprintf(`increase(%s{interface="eth0"}[%dm])`, metric, minutes)
	values, err := queryPrometheus(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	logrus.Debugf("returning %s bytes values: %v", metric, spew.Sprintf("%v", values))

	//adjusting values to 30s
	var adjusted model.Matrix
	for _, r := range values {
		matrix := &model.SampleStream{
			Metric: r.Metric,
		}
		matrix.Values = []model.SamplePair{}
		if len(r.Values) > 0 {
			newValue := r.Values[len(r.Values)-1]
			newValue.Value = model.SampleValue((float64(newValue.Value)) / float64(2*minutes)) // 30s adjustment /2 /minutes
			matrix.Values = append(matrix.Values, newValue)
		}
		adjusted = append(adjusted, matrix)
	}
	return adjusted, nil
}

func queryPrometheus(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, query string) (model.Matrix, error) {
	if query == "" {
		return model.Matrix{}, fmt.Errorf("query cannot be empty")
	}
	adjustedR := r
	adjustedR.Start = adjustedR.End.Add(-60 * time.Second)
	logrus.Debugf("adjusted the start of the prometheus range from %s to %s, to collect a baseline for query %q", r.Start.String(), adjustedR.Start.String(), query)
	values, warnings, err := api.QueryRange(ctx, query, adjustedR)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), nil
}
