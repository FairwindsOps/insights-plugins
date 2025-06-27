package ondemandjobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/insights"
	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/k8s"
	"k8s.io/client-go/kubernetes"
)

type JobConfig struct {
	cronJobName string
	timeout     time.Duration
}

var reportTypeJobConfigMap = map[string]JobConfig{
	"trivy":              {cronJobName: "trivy", timeout: 20 * time.Minute},
	"cloudcosts":         {cronJobName: "cloudcosts", timeout: 5 * time.Minute},
	"falco":              {cronJobName: "falco", timeout: 5 * time.Minute},
	"nova":               {cronJobName: "nova", timeout: 5 * time.Minute},
	"pluto":              {cronJobName: "pluto", timeout: 5 * time.Minute},
	"polaris":            {cronJobName: "polaris", timeout: 5 * time.Minute},
	"prometheus-metrics": {cronJobName: "prometheus-metrics", timeout: 5 * time.Minute},
	"goldilocks":         {cronJobName: "goldilocks", timeout: 5 * time.Minute},
	"rbac-reporter":      {cronJobName: "rbac-reporter", timeout: 5 * time.Minute},
	"right-sizer":        {cronJobName: "right-sizer", timeout: 5 * time.Minute},
	"workloads":          {cronJobName: "workloads", timeout: 5 * time.Minute},
	"kube-hunter":        {cronJobName: "kube-hunter", timeout: 5 * time.Minute},
	"kube-bench":         {cronJobName: "kube-bench", timeout: 5 * time.Minute},
	"kyverno":            {cronJobName: "kyverno", timeout: 5 * time.Minute},
	"gonogo":             {cronJobName: "gonogo", timeout: 5 * time.Minute},
}

func FetchAndProcessOnDemandJobs(insightsClient insights.Client, clientset *kubernetes.Clientset) error {
	onDemandJobs, err := insightsClient.ClaimOnDemandJobs(1)
	if err != nil {
		return fmt.Errorf("failed to fetch on-demand jobs: %w", err)
	}

	if len(onDemandJobs) == 0 {
		slog.Info("no on-demand jobs to process")
		return nil
	}

	for _, onDemandJob := range onDemandJobs {
		err := processOnDemandJob(clientset, onDemandJob)
		if err != nil {
			slog.Error("failed to process on-demand job", "jobID", onDemandJob.ID, "reportType", onDemandJob.ReportType, "error", err)
			err := insightsClient.UpdateOnDemandJobStatus(onDemandJob.ID, insights.JobStatusFailed)
			if err != nil {
				slog.Error("failed to update on-demand job status to failed", "jobID", onDemandJob.ID, "error", err)
			} else {
				slog.Info("updated on-demand job status to failed", "jobID", onDemandJob.ID)
			}
			continue
		}

		err = insightsClient.UpdateOnDemandJobStatus(onDemandJob.ID, insights.JobStatusCompleted)
		if err != nil {
			slog.Error("failed to update on-demand job status to completed", "jobID", onDemandJob.ID, "error", err)
		} else {
			slog.Info("updated on-demand job status to completed", "jobID", onDemandJob.ID)
		}
		slog.Info("processed on-demand job successfully", "jobID", onDemandJob.ID, "reportType", onDemandJob.ReportType)
	}

	return nil
}

func processOnDemandJob(clientset *kubernetes.Clientset, onDemandJob insights.OnDemandJob) error {
	namespace, err := k8s.GetCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	jobConfig, ok := reportTypeJobConfigMap[onDemandJob.ReportType]
	if !ok {
		return fmt.Errorf("unknown report type %s for on-demand job %d", onDemandJob.ReportType, onDemandJob.ID)
	}

	jobName := k8s.GenerateJobName(jobConfig.cronJobName, onDemandJob.ID)
	job, err := k8s.CreateJobFromCronJob(context.TODO(), clientset, namespace, jobConfig.cronJobName, jobName, onDemandJob.OptionsToEnvVars())
	if err != nil {
		return fmt.Errorf("failed to create job from cron job %s: %w", jobConfig.cronJobName, err)
	}

	slog.Info("Created job from cron job", "jobName", job.Name, "cronJobName", jobConfig.cronJobName, "namespace", namespace)

	err = k8s.WaitForJobCompletion(context.TODO(), clientset, job.Namespace, job.Name, jobConfig.timeout)
	if err != nil {
		return fmt.Errorf("job %s/%s did not complete within %s: %w", job.Namespace, job.Name, jobConfig.timeout, err)
	}

	return nil
}
