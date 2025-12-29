package lock

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// PolicySyncLock represents a distributed lock using Kubernetes Lease for preventing concurrent sync operations
type PolicySyncLock struct {
	LeaseName string
	Namespace string
	LockedBy  string
	K8sClient kubernetes.Interface
}

func NewPolicySyncLock(k8sClient kubernetes.Interface) *PolicySyncLock {

	// Get current namespace
	namespace, err := getCurrentNamespace()
	if err != nil {
		slog.Error("Failed to get current namespace", "error", err)
		namespace = "default"
	}

	// Generate unique lock identifier (pod name or job name)
	lockedBy := getLockIdentifier()

	return &PolicySyncLock{
		LeaseName: "kyverno-policy-sync-lock",
		Namespace: namespace,
		LockedBy:  lockedBy,
		K8sClient: k8sClient,
	}
}

// RunWithLeaderElection runs the provided function with leader election using LeaseLock
func (l *PolicySyncLock) RunWithLeaderElection(ctx context.Context, runFunc func(context.Context) error) error {
	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		l.Namespace,
		l.LeaseName,
		l.K8sClient.CoreV1(),
		l.K8sClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: l.LockedBy,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create resource lock: %w", err)
	}

	// Create a cancellable context for leader election - this allows us to cancel it after the sync completes
	leaderCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leadingCtx context.Context) {
				slog.Info("Started leading", "identity", l.LockedBy, "namespace", l.Namespace, "lease", l.LeaseName)
				if err := runFunc(leadingCtx); err != nil {
					slog.Error("Error running leader function", "error", err)
				}
				slog.Info("Sync completed, releasing leadership", "identity", l.LockedBy)
				cancel() // cancel the leader election context to exit after sync completes
			},
			OnStoppedLeading: func() {
				slog.Info("Stopped leading", "identity", l.LockedBy, "namespace", l.Namespace, "lease", l.LeaseName)
			},
			OnNewLeader: func(identity string) {
				if identity == l.LockedBy {
					return
				}
				slog.Info("New leader elected", "identity", identity, "namespace", l.Namespace, "lease", l.LeaseName)
			},
		},
		ReleaseOnCancel: true,
		Name:            l.LeaseName,
	}

	leaderelection.RunOrDie(leaderCtx, leaderElectionConfig)
	return nil
}

// getCurrentNamespace gets the current namespace from environment or service account
func getCurrentNamespace() (string, error) {
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

// getLockIdentifier generates a unique identifier for the lock
func getLockIdentifier() string {
	prefixIdentifier := "fw"
	if podName := os.Getenv("POD_NAME"); podName != "" {
		return fmt.Sprintf("%s-pod-%s", prefixIdentifier, podName)
	}

	if jobName := os.Getenv("JOB_NAME"); jobName != "" {
		return fmt.Sprintf("%s-job-%s", prefixIdentifier, jobName)
	}

	if hostname, err := os.Hostname(); err == nil {
		return fmt.Sprintf("%s-host-%s", prefixIdentifier, hostname)
	}

	return fmt.Sprintf("%s-unknown-%d", prefixIdentifier, time.Now().Unix())
}
