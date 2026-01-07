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
	"time"

	"github.com/prometheus/common/model"
)

// Owner is the information about a pod that a set of metrics belongs to.
type Owner struct {
	Container           string
	PodName             string
	ControllerNamespace string
	ControllerName      string
	ControllerKind      string
}

// Statistics is an aggregation of the metrics by pod/container
type Statistics struct {
	Owner
	StartTime  time.Time
	Metric     string
	Value      int64
	Request    int64
	LimitValue int64
}

// CombinedRequest is the cpu/memory and requests for a given pod/container
type CombinedRequest struct {
	Owner
	cpu             []model.SamplePair
	memory          []model.SamplePair
	memoryRequest   model.SampleValue
	cpuRequest      model.SampleValue
	memoryLimit     model.SampleValue
	cpuLimit        model.SampleValue
	networkTransmit []model.SamplePair
	networkReceive  []model.SamplePair
	storageCapacity []model.SamplePair
	// GPU fields - utilization from DCGM Exporter, requests/limits from kube-state-metrics
	gpu        []model.SamplePair // GPU utilization (0-1 per GPU)
	gpuRequest model.SampleValue  // nvidia.com/gpu requests
	gpuLimit   model.SampleValue  // nvidia.com/gpu limits
}

type NodesMetrics struct {
	IdleCPU        model.SampleValue `json:"IdleCPU,omitempty" yaml:"IdleCPU,omitempty"`
	IdleMemory     model.SampleValue `json:"IdleMemory,omitempty" yaml:"IdleMemory,omitempty"`
	OverheadCPU    model.SampleValue `json:"OverheadCPU,omitempty" yaml:"OverheadCPU,omitempty"`
	OverheadMemory model.SampleValue `json:"OverheadMemory,omitempty" yaml:"OverheadMemory,omitempty"`
}
