package insights

import (
	corev1 "k8s.io/api/core/v1"
)

type OnDemandJobStatus string

const (
	JobStatusPending   OnDemandJobStatus = "pending"
	JobStatusRunning   OnDemandJobStatus = "running"
	JobStatusCompleted OnDemandJobStatus = "completed"
	JobStatusFailed    OnDemandJobStatus = "failed"
)

type OnDemandJob struct {
	ID         int64             `json:"id"`
	ReportType string            `json:"reportType"`
	Status     string            `json:"status"`
	Options    map[string]string `json:"options"`
}

func (odj OnDemandJob) OptionsToEnvVars() []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)
	if odj.Options != nil {
		for key, value := range odj.Options {
			envVars = append(envVars, corev1.EnvVar{
				Name:  ToEnvVarFormat(key),
				Value: value,
			})
		}
	}
	return envVars
}
