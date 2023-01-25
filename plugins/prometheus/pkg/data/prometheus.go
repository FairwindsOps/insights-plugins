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

// getNetworkTransmitBytesIncludingBaseline queries prometheus for
// container_network_transmit_bytes_total, which is a cumulative (total) metric.
// the specified prometheus range will have its start-time expanded to include
// an extra minute, to obtain a baseline to be
// used elsewhere when convert the total values into deltas.
func getNetworkTransmitBytesIncludingBaseline(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	// ifetch: This query temporarily limited to a specific pod.
	// ToDO: SHould we not limit interfaces and combine metrics, or let them all
	// get processed, or provide a config option to specify the desired
	// interface (defaulting to eth0)?
	query := `container_network_transmit_bytes_total{pod="testapp-5f6778868d-wbdj2",interface!="tunl0",interface!="ip6tnl0"}`
	values, err := queryIncludingExtraLeadingMinuteOfData(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	logrus.Debugf("returning network transmit bytes values: %v", spew.Sprintf("%v", values))
	return values, nil
}

// getNetworkReceiveBytesIncludingBaseline queries prometheus for
// container_network_receive_bytes_total, which is a cumulative (total) metric.
// the specified prometheus range will have its start-time expanded to include
// an extra minute, to obtain a baseline to be
// used elsewhere when convert the total values into deltas.
func getNetworkReceiveBytesIncludingBaseline(ctx context.Context, api prometheusV1.API, r prometheusV1.Range) (model.Matrix, error) {
	// ifetch: This query temporarily limited to a specific pod.
	// ToDO: SHould we not limit interfaces and combine metrics, or let them all
	// get processed, or provide a config option to specify the desired
	// interface (defaulting to eth0)?
	query := `container_network_receive_bytes_total{pod="testapp-5f6778868d-wbdj2",interface!="tunl0",interface!="ip6tnl0"}`
	values, err := queryIncludingExtraLeadingMinuteOfData(ctx, api, r, query)
	if err != nil {
		return model.Matrix{}, err
	}
	logrus.Debugf("returning network transmit bytes values: %v", spew.Sprintf("%v", values))
	return values, nil
}

// queryIncludingExtraLeadingMinuteOfData queries prometheus while extending
// the beginning of the prometheus range to include an extra minute of
// metrics. This is useful to obtain a baseline metric when converting total;
// cumulitive metrics to ddelta metrics.
func queryIncludingExtraLeadingMinuteOfData(ctx context.Context, api prometheusV1.API, r prometheusV1.Range, query string) (model.Matrix, error) {
	if query == "" {
		return model.Matrix{}, fmt.Errorf("query cannot be empty")
	}
	adjustedR := r
	adjustedR.Start = adjustedR.Start.Add(-60 * time.Second) // adjust by a full minute to not scue all results
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
