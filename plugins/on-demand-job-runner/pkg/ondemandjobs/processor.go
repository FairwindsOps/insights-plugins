package ondemandjobs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/insights"
	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/k8s"
	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/kyverno"
	"k8s.io/client-go/kubernetes"
)

type JobConfig struct {
	cronJobName string
	timeout     time.Duration
}

var reportTypeJobConfigMap = map[string]JobConfig{
	"trivy":               {cronJobName: "trivy", timeout: 20 * time.Minute},
	"cloudcosts":          {cronJobName: "cloudcosts", timeout: 5 * time.Minute},
	"falco":               {cronJobName: "falco", timeout: 5 * time.Minute},
	"nova":                {cronJobName: "nova", timeout: 5 * time.Minute},
	"pluto":               {cronJobName: "pluto", timeout: 5 * time.Minute},
	"polaris":             {cronJobName: "polaris", timeout: 5 * time.Minute},
	"prometheus-metrics":  {cronJobName: "prometheus-metrics", timeout: 5 * time.Minute},
	"goldilocks":          {cronJobName: "goldilocks", timeout: 5 * time.Minute},
	"rbac-reporter":       {cronJobName: "rbac-reporter", timeout: 5 * time.Minute},
	"right-sizer":         {cronJobName: "right-sizer", timeout: 5 * time.Minute},
	"workloads":           {cronJobName: "workloads", timeout: 5 * time.Minute},
	"kube-hunter":         {cronJobName: "kube-hunter", timeout: 5 * time.Minute},
	"kube-bench":          {cronJobName: "kube-bench", timeout: 5 * time.Minute},
	"kyverno":             {cronJobName: "kyverno", timeout: 5 * time.Minute},
	"kyverno-policy-sync": {cronJobName: "kyverno-policy-sync", timeout: 10 * time.Minute},
	"gonogo":              {cronJobName: "gonogo", timeout: 5 * time.Minute},
}

// FetchAndProcessOnDemandJobs fetches on-demand jobs from the insights client and processes them concurrently.
func FetchAndProcessOnDemandJobs(insightsClient insights.Client, clientset *kubernetes.Clientset, maxConcurrentJobs int) error {
	onDemandJobs, err := insightsClient.ClaimOnDemandJobs(maxConcurrentJobs)
	if err != nil {
		return fmt.Errorf("failed to fetch on-demand jobs: %w", err)
	}

	if len(onDemandJobs) == 0 {
		slog.Info("no on-demand jobs to process")
		return nil
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentJobs)
	for _, onDemandJob := range onDemandJobs {
		wg.Add(1)
		semaphore <- struct{}{} // acquire

		go func(job insights.OnDemandJob) {
			defer wg.Done()
			defer func() { <-semaphore }() // release

			err := processOnDemandJob(clientset, job)
			if err != nil {
				slog.Error("failed to process on-demand job", "jobID", job.ID, "reportType", job.ReportType, "error", err)
				if updateErr := insightsClient.UpdateOnDemandJobStatus(job.ID, insights.JobStatusFailed); updateErr != nil {
					slog.Error("failed to update on-demand job status to failed", "jobID", job.ID, "error", updateErr)
				} else {
					slog.Info("updated on-demand job status to failed", "jobID", job.ID)
				}
				return
			}

			if updateErr := insightsClient.UpdateOnDemandJobStatus(job.ID, insights.JobStatusCompleted); updateErr != nil {
				slog.Error("failed to update on-demand job status to completed", "jobID", job.ID, "error", updateErr)
			} else {
				slog.Info("updated on-demand job status to completed", "jobID", job.ID)
			}

			slog.Info("processed on-demand job successfully", "jobID", job.ID, "reportType", job.ReportType)
		}(onDemandJob)
	}

	wg.Wait()
	return nil
}

func processOnDemandJob(clientset *kubernetes.Clientset, onDemandJob insights.OnDemandJob) error {
	// Special handling for kyverno-policy-sync job type
	if onDemandJob.ReportType == "kyverno-policy-sync" {
		return processKyvernoPolicySyncJob(clientset, onDemandJob)
	}

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

// processKyvernoPolicySyncJob handles the special case of kyverno-policy-sync jobs
func processKyvernoPolicySyncJob(clientset *kubernetes.Clientset, onDemandJob insights.OnDemandJob) error {
	slog.Info("Processing Kyverno policy sync job", "jobID", onDemandJob.ID)

	// Create insights client
	insightsClient := insights.NewClient(
		os.Getenv("FAIRWINDS_INSIGHTS_HOST"),
		os.Getenv("FAIRWINDS_TOKEN"),
		os.Getenv("FAIRWINDS_ORG"),
		os.Getenv("FAIRWINDS_CLUSTER"),
		os.Getenv("FAIRWINDS_DEV_MODE") == "true",
	)

	// Parse configuration from job options
	config := kyverno.PolicySyncConfig{
		DryRun:           onDemandJob.Options["dryRun"] == "true",
		SyncInterval:     15 * time.Minute,
		LockTimeout:      30 * time.Minute,
		ValidatePolicies: onDemandJob.Options["validatePolicies"] != "false",
	}

	// Create dynamic client
	dynamicClient, err := k8s.GetDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create policy sync processor
	processor := kyverno.NewPolicySyncProcessor(insightsClient, clientset, dynamicClient, config)

	// Execute policy sync
	result, err := processor.SyncPolicies(context.TODO())
	if err != nil {
		return fmt.Errorf("policy sync failed: %w", err)
	}

	// Log results
	slog.Info("Policy sync completed",
		"success", result.Success,
		"duration", result.Duration,
		"summary", result.Summary,
		"applied", len(result.Applied),
		"updated", len(result.Updated),
		"removed", len(result.Removed),
		"failed", len(result.Failed))

	if !result.Success {
		return fmt.Errorf("policy sync failed: %v", result.Errors)
	}

	return nil
}
