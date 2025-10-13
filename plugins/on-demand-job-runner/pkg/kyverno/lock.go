package kyverno

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Acquire acquires the policy sync lock using Kubernetes ConfigMap
func (l *PolicySyncLock) Acquire() error {
	ctx := context.Background()

	// Check if lock ConfigMap exists and is not stale
	if l.isLockStale(ctx) {
		// Remove stale lock
		l.Release()
	}

	// Try to create lock ConfigMap
	lockData := map[string]string{
		"locked-by":    l.LockedBy,
		"locked-at":    time.Now().UTC().Format(time.RFC3339),
		"lock-timeout": l.LockTimeout.String(),
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      l.ConfigMapName,
			Namespace: l.Namespace,
			Labels: map[string]string{
				"app":                    "kyverno-policy-sync",
				"insights.fairwinds.com": "policy-sync-lock",
			},
		},
		Data: lockData,
	}

	_, err := l.K8sClient.CoreV1().ConfigMaps(l.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("policy sync is already running (lock ConfigMap exists: %s/%s)", l.Namespace, l.ConfigMapName)
		}
		return fmt.Errorf("failed to create lock ConfigMap: %w", err)
	}

	slog.Info("Acquired policy sync lock", "lockedBy", l.LockedBy, "namespace", l.Namespace, "configMap", l.ConfigMapName)
	return nil
}

// Release releases the policy sync lock by deleting the ConfigMap
func (l *PolicySyncLock) Release() error {
	ctx := context.Background()

	err := l.K8sClient.CoreV1().ConfigMaps(l.Namespace).Delete(ctx, l.ConfigMapName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to release lock ConfigMap: %w", err)
	}

	slog.Info("Released policy sync lock", "lockedBy", l.LockedBy, "namespace", l.Namespace, "configMap", l.ConfigMapName)
	return nil
}

// isLockStale checks if the lock ConfigMap is stale (older than lock timeout)
func (l *PolicySyncLock) isLockStale(ctx context.Context) bool {
	configMap, err := l.K8sClient.CoreV1().ConfigMaps(l.Namespace).Get(ctx, l.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false // ConfigMap doesn't exist, not stale
		}
		slog.Warn("Failed to check lock ConfigMap", "error", err)
		return false
	}

	// Check if lock is older than timeout
	lockedAtStr, exists := configMap.Data["locked-at"]
	if !exists {
		slog.Warn("Lock ConfigMap missing locked-at timestamp, considering stale")
		return true
	}

	lockedAt, err := time.Parse(time.RFC3339, lockedAtStr)
	if err != nil {
		slog.Warn("Failed to parse lock timestamp, considering stale", "timestamp", lockedAtStr, "error", err)
		return true
	}

	isStale := time.Since(lockedAt) > l.LockTimeout
	if isStale {
		slog.Info("Lock is stale, will be removed", "lockedAt", lockedAt, "timeout", l.LockTimeout)
	}

	return isStale
}
