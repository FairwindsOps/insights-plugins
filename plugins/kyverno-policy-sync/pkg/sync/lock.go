package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

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
