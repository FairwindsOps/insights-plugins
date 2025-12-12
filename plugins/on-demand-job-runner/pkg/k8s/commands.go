package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to local kubeconfig
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config from in-cluster config or kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

func CreateJobFromCronJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, cronJobName, newJobName string, extraEnv []corev1.EnvVar, backoffLimit *int32) (*batchv1.Job, error) {
	cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, cronJobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CronJob %s in namespace %s: %w", cronJobName, namespace, err)
	}

	// Construct Job from CronJob spec
	jobSpec := cronJob.Spec.JobTemplate.Spec
	jobSpec.TTLSecondsAfterFinished = lo.ToPtr(int32(10 * 60)) // Set TTL to 5 minutes
	jobSpec.BackoffLimit = backoffLimit
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newJobName,
			Namespace: namespace,
		},
		Spec: jobSpec,
	}

	// Add/override environment variables
	for i, container := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = mergeEnvs(container.Env, extraEnv)
	}

	return clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
}

func WaitForJobCompletion(ctx context.Context, clientset *kubernetes.Clientset, namespace, jobName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for job %s to complete", jobName)
		}

		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting job: %w", err)
		}

		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				slog.Info("job completed successfully", "jobName", jobName, "namespace", namespace)
				return nil
			}
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				return fmt.Errorf("job failed: %s", cond.Reason)
			}
		}

		slog.Info("waiting for job to complete...", "jobName", jobName, "namespace", namespace)
		time.Sleep(5 * time.Second)
	}
}

func GetCurrentNamespace() (string, error) {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			// fallback to env variable
			namespace := os.Getenv("NAMESPACE")
			if namespace != "" {
				return namespace, nil
			}
			return "", fmt.Errorf("namespace file not found and NAMESPACE env variable is not set")
		}
		return "", fmt.Errorf("failed to read namespace file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
