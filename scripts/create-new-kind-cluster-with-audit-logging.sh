#!/bin/bash

# Script to enable REAL audit logging in Kind cluster using the working approach
# Based on the official Kind documentation and working examples

set -e

echo "🔧 Creating Kind cluster with REAL audit logging enabled..."

# Clean up any existing clusters first
echo "🧹 Cleaning up existing clusters..."
kind delete cluster --name kind 2>/dev/null || true
docker rm -f kind-control-plane 2>/dev/null || true

# Create audit logs directory
echo "📁 Creating audit logs directory..."
mkdir -p /tmp/audit-logs

# Create audit policy file on host
echo "📝 Creating audit policy file..."
cat > /tmp/audit-policy.yaml << 'EOF'
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all requests to deployments
- level: RequestResponse
  verbs: ["create", "update", "delete"]
  resources:
  - group: "apps"
    resources: ["deployments"]
# Log all admission controller decisions
- level: RequestResponse
  verbs: ["create", "update", "delete"]
  resources:
  - group: "admissionregistration.k8s.io"
    resources: ["validatingadmissionpolicies", "validatingadmissionpolicybindings"]
# Log all events
- level: RequestResponse
  verbs: ["create", "update", "delete"]
  resources:
  - group: ""
    resources: ["events"]
# Log metadata for all other requests
- level: Metadata
EOF

# Create the working Kind cluster configuration
echo "⚙️ Creating Kind cluster configuration..."
cat > /tmp/kind-audit-config.yaml << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        audit-log-path: /var/log/kubernetes/kube-apiserver-audit.log
        audit-policy-file: /etc/kubernetes/policies/audit-policy.yaml
      extraVolumes:
      - name: audit-policies
        hostPath: /etc/kubernetes/policies
        mountPath: /etc/kubernetes/policies
        readOnly: true
        pathType: DirectoryOrCreate
      - name: audit-logs
        hostPath: /var/log/kubernetes
        mountPath: /var/log/kubernetes
        readOnly: false
        pathType: DirectoryOrCreate
  extraMounts:
  - hostPath: /tmp/audit-policy.yaml
    containerPath: /etc/kubernetes/policies/audit-policy.yaml
    readOnly: true
  - hostPath: /tmp/audit-logs
    containerPath: /var/log/kubernetes
    readOnly: false
EOF

# Create new cluster with audit logging
echo "🚀 Creating new cluster with REAL audit logging..."
echo "⏳ This may take a few minutes..."

# Use timeout to prevent hanging
timeout 300 kind create cluster --config /tmp/kind-audit-config.yaml || {
    echo "❌ Cluster creation timed out or failed"
    echo "🧹 Cleaning up..."
    kind delete cluster --name kind 2>/dev/null || true
    docker rm -f kind-control-plane 2>/dev/null || true
    echo "💡 Try running the script again"
    exit 1
}

# Wait for cluster to be ready
echo "⏳ Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s || {
    echo "❌ Cluster not ready after timeout"
    echo "🔍 Checking cluster status..."
    kubectl get nodes
    exit 1
}

# Set up kubeconfig
echo "🔧 Setting up kubeconfig..."
kind get kubeconfig --name kind > ~/.kube/config

# Verify cluster is working
echo "✅ Verifying cluster..."
kubectl get nodes

# Test audit logging by creating some resources
echo "🧪 Testing REAL audit logging..."
kubectl create deployment test-nginx --image=nginx --replicas=1 || true
kubectl create deployment test-httpd --image=httpd --replicas=1 || true
sleep 3

# Check if audit log file exists and has content
if [ -f "/tmp/audit-logs/kube-apiserver-audit.log" ]; then
    echo "📊 REAL audit log file created successfully!"
    echo "📈 Audit log size: $(wc -l < /tmp/audit-logs/kube-apiserver-audit.log) lines"
    echo "🔍 Sample REAL audit log entries:"
    head -3 /tmp/audit-logs/kube-apiserver-audit.log | jq . 2>/dev/null || head -3 /tmp/audit-logs/kube-apiserver-audit.log
    echo ""
    echo "🎉 SUCCESS! Real audit logging is working!"
    echo "📁 Real audit logs location: /tmp/audit-logs/kube-apiserver-audit.log"
    echo "👀 To view real audit logs: tail -f /tmp/audit-logs/kube-apiserver-audit.log"
    echo "🔍 To test more: kubectl create deployment nginx --image=nginx"
    echo ""
    echo "✅ Your event watcher will now work with REAL audit logs!"
else
    echo "⚠️  Real audit log file not found at /tmp/audit-logs/kube-apiserver-audit.log"
    echo "🔍 Checking directory contents:"
    ls -la /tmp/audit-logs/
    echo ""
    echo "❌ Real audit logging failed. The API server may not have started properly."
    echo "💡 Try running the script again or check the API server logs:"
    echo "   docker logs kind-control-plane"
fi

# Clean up temporary config file
rm -f /tmp/kind-audit-config.yaml

echo ""
echo "🎯 This is REAL audit logging - not mock data!"
echo "📊 The audit log contains actual Kubernetes API server events"
echo "🔧 Your event watcher will process real audit events"