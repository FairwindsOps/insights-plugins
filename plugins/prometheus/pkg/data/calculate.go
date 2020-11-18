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
	"time"
)

// CalculateStatistics finds the max/min/avg for a set of data points by hour
func CalculateStatistics(values []CombinedRequest) []Statistics {
	stats := make([]Statistics, 0)
	type memCPU struct {
		memoryArray []float64
		cpuArray    []float64
	}
	for _, value := range values {
		for _, mem := range value.memory {
			timestamp := time.Unix(int64(mem.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     "Memory",
				Value:      int64(mem.Value),
				Request:    int64(value.memoryRequest),
				LimitValue: int64(value.memoryLimit),
			})

		}
		for _, cpu := range value.cpu {
			timestamp := time.Unix(int64(cpu.Timestamp)/1000, 0)
			stats = append(stats, Statistics{
				StartTime:  timestamp,
				Owner:      value.Owner,
				Metric:     "CPU",
				Value:      int64(cpu.Value * 1000),
				Request:    int64(value.cpuRequest * 1000),
				LimitValue: int64(value.cpuLimit * 1000),
			})
		}

	}

	return stats
}
