// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package data

import (
	"context"

	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

func getMemory(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	query := `container_memory_usage_bytes{image!="", id=~"/kubepods.*", container!="POD", container!=""}`
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
	query := `kube_pod_container_resource_requests_memory_bytes{container!="POD", container!=""}`
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
	query := `kube_pod_container_resource_requests_cpu_cores{container!="POD", container!=""}`
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
	query := `kube_pod_container_resource_limits_memory_bytes{container!="POD", container!=""}`
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
	query := `kube_pod_container_resource_limits_cpu_cores{container!="POD", container!=""}`
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
	query := `rate(container_cpu_usage_seconds_total{image!="", id=~"/kubepods.*", container!="POD", container!=""}[2m])`
	values, warnings, err := api.QueryRange(ctx, query, r)
	for _, warning := range warnings {
		logrus.Warn(warning)
	}
	if err != nil {
		return model.Matrix{}, err
	}
	return values.(model.Matrix), err
}
