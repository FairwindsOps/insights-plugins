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
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestCalculateStatistics_GPUMetrics(t *testing.T) {
	// Test data matching the expected data flow:
	// Prometheus (DCGM: 85%) → PromQL /100 (0.85) → CombinedRequest (0.85) → calculate.go ×100 (85)
	// Prometheus (KSM: 2 GPUs) → CombinedRequest (2.0) → calculate.go ×1000 (2000)
	testCases := []struct {
		name               string
		gpuUsage           float64 // 0-1 (normalized from PromQL)
		gpuRequest         float64 // GPU count (e.g., 2.0)
		gpuLimit           float64 // GPU count (e.g., 4.0)
		expectedValue      int64   // 0-100 (percentage)
		expectedRequest    int64   // milli-GPUs
		expectedLimitValue int64   // milli-GPUs
	}{
		{
			name:               "typical GPU usage 85%",
			gpuUsage:           0.85, // 85% utilization normalized to 0-1
			gpuRequest:         2.0,  // 2 GPUs requested
			gpuLimit:           2.0,  // 2 GPUs limit
			expectedValue:      85,   // 0.85 * 100 = 85
			expectedRequest:    2000, // 2.0 * 1000 = 2000 milli-GPUs
			expectedLimitValue: 2000, // 2.0 * 1000 = 2000 milli-GPUs
		},
		{
			name:               "low GPU usage 10%",
			gpuUsage:           0.10,
			gpuRequest:         1.0,
			gpuLimit:           1.0,
			expectedValue:      10,
			expectedRequest:    1000,
			expectedLimitValue: 1000,
		},
		{
			name:               "full GPU usage 100%",
			gpuUsage:           1.0, // 100% utilization
			gpuRequest:         4.0,
			gpuLimit:           8.0,
			expectedValue:      100,
			expectedRequest:    4000,
			expectedLimitValue: 8000,
		},
		{
			name:               "zero GPU usage",
			gpuUsage:           0.0,
			gpuRequest:         1.0,
			gpuLimit:           1.0,
			expectedValue:      0,
			expectedRequest:    1000,
			expectedLimitValue: 1000,
		},
		{
			name:               "fractional GPU (time-sliced) 0.5 GPU",
			gpuUsage:           0.75,
			gpuRequest:         0.5, // nvidia.com/gpu.shared allows fractional GPUs
			gpuLimit:           0.5,
			expectedValue:      75,
			expectedRequest:    500, // 0.5 * 1000 = 500 milli-GPUs
			expectedLimitValue: 500,
		},
		{
			name:               "multiple GPUs with high usage",
			gpuUsage:           0.92,
			gpuRequest:         8.0, // 8 GPU training job
			gpuLimit:           8.0,
			expectedValue:      92,
			expectedRequest:    8000,
			expectedLimitValue: 8000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timestamp := model.Time(1674153900000) // Unix timestamp in milliseconds

			input := []CombinedRequest{
				{
					Owner: Owner{
						Container:           "cuda-container",
						PodName:             "ml-training-abc123",
						ControllerNamespace: "ml-workloads",
						ControllerName:      "ml-training",
						ControllerKind:      "Deployment",
					},
					gpu: []model.SamplePair{
						{Timestamp: timestamp, Value: model.SampleValue(tc.gpuUsage)},
					},
					gpuRequest: model.SampleValue(tc.gpuRequest),
					gpuLimit:   model.SampleValue(tc.gpuLimit),
				},
			}

			stats := CalculateStatistics(input)

			// Find the GPU statistic
			var gpuStat *Statistics
			for i := range stats {
				if stats[i].Metric == MetricGPU {
					gpuStat = &stats[i]
					break
				}
			}

			assert.NotNil(t, gpuStat, "GPU statistic should be present")
			assert.Equal(t, tc.expectedValue, gpuStat.Value, "GPU usage should be scaled to 0-100 percentage")
			assert.Equal(t, tc.expectedRequest, gpuStat.Request, "GPU request should be scaled to milli-GPUs")
			assert.Equal(t, tc.expectedLimitValue, gpuStat.LimitValue, "GPU limit should be scaled to milli-GPUs")

			// Verify owner fields are preserved
			assert.Equal(t, "cuda-container", gpuStat.Container)
			assert.Equal(t, "ml-training-abc123", gpuStat.PodName)
			assert.Equal(t, "ml-workloads", gpuStat.ControllerNamespace)
			assert.Equal(t, "ml-training", gpuStat.ControllerName)
			assert.Equal(t, "Deployment", gpuStat.ControllerKind)
		})
	}
}

func TestCalculateStatistics_NoGPUMetrics(t *testing.T) {
	// Test that pods without GPU don't produce GPU statistics
	input := []CombinedRequest{
		{
			Owner: Owner{
				Container:           "web",
				PodName:             "web-server-123",
				ControllerNamespace: "default",
				ControllerName:      "web-server",
				ControllerKind:      "Deployment",
			},
			cpu: []model.SamplePair{
				{Timestamp: model.Time(1674153900000), Value: 0.5},
			},
			cpuRequest: 1.0,
			cpuLimit:   2.0,
			memory: []model.SamplePair{
				{Timestamp: model.Time(1674153900000), Value: 134217728},
			},
			memoryRequest: 268435456,
			memoryLimit:   536870912,
			// No GPU fields set
		},
	}

	stats := CalculateStatistics(input)

	// Should have CPU and Memory stats, but not GPU
	var hasGPU bool
	for _, stat := range stats {
		if stat.Metric == MetricGPU {
			hasGPU = true
			break
		}
	}

	assert.False(t, hasGPU, "Pod without GPU should not produce GPU statistics")
	assert.True(t, len(stats) >= 2, "Should have at least CPU and Memory statistics")
}

func TestCalculateStatistics_GPURequestLimitWithoutUsage(t *testing.T) {
	// Pod has GPU request/limit from kube-state-metrics but no GPU utilization exporter (value.gpu empty).
	// We should still emit one GPU stat so request/limit appear in output.
	input := []CombinedRequest{
		{
			Owner: Owner{
				Container:           "cuda-container",
				PodName:             "ml-pod-abc",
				ControllerNamespace: "default",
				ControllerName:      "ml-job",
				ControllerKind:      "Job",
			},
			cpu: []model.SamplePair{
				{Timestamp: model.Time(1674153900000), Value: 0.5},
			},
			cpuRequest:  2.0,
			cpuLimit:    4.0,
			memory:      []model.SamplePair{{Timestamp: model.Time(1674153900000), Value: 1e9}},
			memoryRequest: 2e9,
			memoryLimit:   4e9,
			gpu:         nil, // no utilization exporter
			gpuRequest:  2.0, // from kube-state-metrics
			gpuLimit:    2.0,
		},
	}

	stats := CalculateStatistics(input)

	var gpuStat *Statistics
	for i := range stats {
		if stats[i].Metric == MetricGPU {
			gpuStat = &stats[i]
			break
		}
	}
	assert.NotNil(t, gpuStat, "Should emit GPU stat when request/limit present even without usage")
	assert.Equal(t, int64(0), gpuStat.Value, "Value should be 0 when no utilization data")
	assert.Equal(t, int64(2000), gpuStat.Request, "Request should be 2 GPUs → 2000 milli-GPUs")
	assert.Equal(t, int64(2000), gpuStat.LimitValue, "LimitValue should be 2 GPUs → 2000 milli-GPUs")
	assert.Equal(t, "cuda-container", gpuStat.Container)
	assert.Equal(t, "ml-pod-abc", gpuStat.PodName)
}

func TestCalculateStatistics_MultipleTimestamps(t *testing.T) {
	// Test GPU metrics with multiple time samples (typical scrape pattern)
	timestamps := []model.Time{
		model.Time(1674153900000),
		model.Time(1674153930000),
		model.Time(1674153960000),
	}
	usageValues := []float64{0.80, 0.85, 0.90} // Increasing GPU usage

	gpuSamples := make([]model.SamplePair, len(timestamps))
	for i, ts := range timestamps {
		gpuSamples[i] = model.SamplePair{
			Timestamp: ts,
			Value:     model.SampleValue(usageValues[i]),
		}
	}

	input := []CombinedRequest{
		{
			Owner: Owner{
				Container:           "trainer",
				PodName:             "training-job-xyz",
				ControllerNamespace: "ml",
				ControllerName:      "training-job",
				ControllerKind:      "Job",
			},
			gpu:        gpuSamples,
			gpuRequest: 2.0,
			gpuLimit:   2.0,
		},
	}

	stats := CalculateStatistics(input)

	// Should have one GPU statistic per timestamp
	gpuStats := make([]Statistics, 0)
	for _, stat := range stats {
		if stat.Metric == "GPU" {
			gpuStats = append(gpuStats, stat)
		}
	}

	assert.Equal(t, 3, len(gpuStats), "Should have one GPU stat per timestamp")

	// Verify values are correctly scaled
	expectedValues := []int64{80, 85, 90}
	for i, stat := range gpuStats {
		assert.Equal(t, expectedValues[i], stat.Value, "GPU usage at timestamp %d should be %d", i, expectedValues[i])
		assert.Equal(t, int64(2000), stat.Request, "GPU request should be consistent")
		assert.Equal(t, int64(2000), stat.LimitValue, "GPU limit should be consistent")
	}
}

func TestCalculateStatistics_CPUScalingComparison(t *testing.T) {
	// Verify GPU scaling follows similar patterns to CPU (both use milli-units for precision)
	// CPU: cores → millicores (×1000)
	// GPU requests/limits: GPU count → milli-GPUs (×1000)
	// GPU usage: 0-1 float → 0-100 integer (percentage)

	input := []CombinedRequest{
		{
			Owner: Owner{
				Container:           "compute",
				PodName:             "compute-pod-1",
				ControllerNamespace: "default",
				ControllerName:      "compute",
				ControllerKind:      "Deployment",
			},
			cpu: []model.SamplePair{
				{Timestamp: model.Time(1674153900000), Value: 0.5}, // 0.5 cores
			},
			cpuRequest: 1.0, // 1 core
			cpuLimit:   2.0, // 2 cores
			gpu: []model.SamplePair{
				{Timestamp: model.Time(1674153900000), Value: 0.75}, // 75% GPU usage
			},
			gpuRequest: 1.0, // 1 GPU
			gpuLimit:   2.0, // 2 GPUs
		},
	}

	stats := CalculateStatistics(input)

	var cpuStat, gpuStat *Statistics
	for i := range stats {
		if stats[i].Metric == "CPU" {
			cpuStat = &stats[i]
		}
		if stats[i].Metric == "GPU" {
			gpuStat = &stats[i]
		}
	}

	assert.NotNil(t, cpuStat, "CPU stat should exist")
	assert.NotNil(t, gpuStat, "GPU stat should exist")

	// CPU: 0.5 cores → 500 millicores, request 1.0 → 1000, limit 2.0 → 2000
	assert.Equal(t, int64(500), cpuStat.Value, "CPU value in millicores")
	assert.Equal(t, int64(1000), cpuStat.Request, "CPU request in millicores")
	assert.Equal(t, int64(2000), cpuStat.LimitValue, "CPU limit in millicores")

	// GPU: 0.75 → 75%, request 1.0 → 1000 milli-GPUs, limit 2.0 → 2000 milli-GPUs
	assert.Equal(t, int64(75), gpuStat.Value, "GPU usage as percentage")
	assert.Equal(t, int64(1000), gpuStat.Request, "GPU request in milli-GPUs")
	assert.Equal(t, int64(2000), gpuStat.LimitValue, "GPU limit in milli-GPUs")
}
