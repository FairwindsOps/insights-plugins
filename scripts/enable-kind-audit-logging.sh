#!/bin/bash

# Script to enable audit logging in Kind cluster
# This requires recreating the cluster with audit logging enabled

echo "Creating Kind cluster with audit logging enabled..."

# Create a new Kind cluster configuration with audit logging
cat > kind-audit-config.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
  - hostPath: /tmp/audit-logs
    containerPath: /var/log/audit
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        audit-log-path: /var/log/audit/audit.log
        audit-policy-file: /etc/kubernetes/audit-policy.yaml
        audit-log-maxage: "30"
        audit-log-maxbackup: "3"
        audit-log-maxsize: "100"
      extraVolumes:
      - name: audit-policy
        hostPath: /tmp/audit-policy.yaml
        mountPath: /etc/kubernetes/audit-policy.yaml
        readOnly: true
        pathType: File
      - name: audit-logs
        hostPath: /tmp/audit-logs
        mountPath: /var/log/audit
        pathType: DirectoryOrCreate
EOF

# Create audit policy file on host
cp audit-policy.yaml /tmp/audit-policy.yaml

# Create audit logs directory
mkdir -p /tmp/audit-logs

# Delete existing cluster if it exists
kind delete cluster --name kind

# Create new cluster with audit logging
kind create cluster --config kind-audit-config.yaml

echo "Kind cluster created with audit logging enabled!"
echo "Audit logs will be written to: /tmp/audit-logs/audit.log"
echo "To view audit logs: tail -f /tmp/audit-logs/audit.log"
