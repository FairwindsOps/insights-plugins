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
)

// CalculateStatistics finds the max/min/avg for a set of data points by hour
func CalculateStatistics(values []CombinedRequest) []Statistics {
	stats := make([]Statistics, 0)
	for _, value := range values {
		for _, mem := range value.memory {
			timestamp := time.Unix(int64(mem.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     MetricMemory,
				Value:      int64(mem.Value),
				Request:    int64(value.memoryRequest),
				LimitValue: int64(value.memoryLimit),
			})

		}
		for _, cpu := range value.cpu {
			timestamp := time.Unix(int64(cpu.Timestamp)/1000, 0)
			var cpuValue int64
			if cpu.Value > 0 && cpu.Value < 0.001 {
				cpuValue = 1
			} else {
				cpuValue = int64(cpu.Value * 1000)
			}
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     MetricCPU,
				Value:      cpuValue,
				Request:    int64(value.cpuRequest * 1000),
				LimitValue: int64(value.cpuLimit * 1000),
			})
		}

		for _, networkTransmit := range value.networkTransmit {
			timestamp := time.Unix(int64(networkTransmit.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime: timestamp,
				Owner:     value.Owner,
				Metric:    MetricNetworkTransmit,
				Value:     int64(networkTransmit.Value),
			})
		}

		for _, networkReceive := range value.networkReceive {
			timestamp := time.Unix(int64(networkReceive.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime: timestamp,
				Owner:     value.Owner,
				Metric:    MetricNetworkReceive,
				Value:     int64(networkReceive.Value),
			})
		}

		for _, storageCapacity := range value.storageCapacity {
			timestamp := time.Unix(int64(storageCapacity.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime: timestamp,
				Owner:     value.Owner,
				Metric:    MetricStorageCapacity,
				Value:     int64(storageCapacity.Value),
			})
		}

		// GPU metrics
		// - Usage: 0-1 float from PromQL → 0-100 integer (percentage)
		// - Request/Limit: GPU count → milli-GPUs (×1000) for fractional GPU support
		// Emit GPU stats when we have usage samples and/or when we have request/limit from kube-state-metrics
		// (so GPU request/limit appear in output even when no GPU utilization exporter is installed).
		hasGPURequestOrLimit := value.gpuRequest > 0 || value.gpuLimit > 0
		for _, gpu := range value.gpu {
			timestamp := time.Unix(int64(gpu.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     MetricGPU,
				Value:      int64(gpu.Value * 100),         // GPU utilization 0-1 → 0-100 (percentage)
				Request:    int64(value.gpuRequest * 1000),  // GPU count → milli-GPUs
				LimitValue: int64(value.gpuLimit * 1000),   // GPU count → milli-GPUs
			})
		}
		if len(value.gpu) == 0 && hasGPURequestOrLimit {
			// No utilization exporter; still emit one GPU stat so request/limit appear in output.
			var timestamp time.Time
			if len(value.cpu) > 0 {
				timestamp = time.Unix(int64(value.cpu[0].Timestamp)/1000, 0)
			} else if len(value.memory) > 0 {
				timestamp = time.Unix(int64(value.memory[0].Timestamp)/1000, 0)
			} else {
				timestamp = time.Now().Truncate(time.Minute)
			}
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     MetricGPU,
				Value:      0,                               // no utilization data
				Request:    int64(value.gpuRequest * 1000),  // GPU count → milli-GPUs
				LimitValue: int64(value.gpuLimit * 1000),    // GPU count → milli-GPUs
			})
		}

	}

	return stats
}
