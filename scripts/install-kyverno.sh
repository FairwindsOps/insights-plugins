#!/bin/bash

# Script to install Kyverno in Kind cluster
set -e

echo "🔧 Installing Kyverno in Kind cluster..."

# Check if kubectl is available and cluster is accessible
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "❌ No accessible Kubernetes cluster found"
    echo "💡 Make sure you have a Kind cluster running: kind create cluster"
    exit 1
fi

echo "✅ Cluster is accessible"

# Install Kyverno using the official installation method
echo "📦 Installing Kyverno..."

# Method 1: Try using the official Kyverno installation
if kubectl apply -f https://github.com/kyverno/kyverno/releases/download/v1.12.0/install.yaml; then
    echo "✅ Kyverno installed successfully using official method"
else
    echo "⚠️  Official installation failed, trying alternative method..."
    
    # Method 2: Install using Helm (if available)
    if command -v helm >/dev/null 2>&1; then
        echo "📦 Installing Kyverno using Helm..."
        helm repo add kyverno https://kyverno.github.io/kyverno/
        helm repo update
        helm install kyverno kyverno/kyverno -n kyverno --create-namespace
        echo "✅ Kyverno installed successfully using Helm"
    else
        echo "❌ Both installation methods failed"
        echo "💡 Please install Kyverno manually:"
        echo "   kubectl apply -f https://github.com/kyverno/kyverno/releases/download/v1.12.0/install.yaml"
        exit 1
    fi
fi

# Wait for Kyverno to be ready
echo "⏳ Waiting for Kyverno to be ready..."
kubectl wait --for=condition=Ready pods -l app.kubernetes.io/name=kyverno -n kyverno --timeout=300s

# Verify installation
echo "🔍 Verifying Kyverno installation..."
kubectl get pods -n kyverno
kubectl get crd | grep kyverno

echo ""
echo "🎉 Kyverno installation complete!"
echo "📋 Available CRDs:"
kubectl get crd | grep kyverno

echo ""
echo "✅ Your Kyverno policy sync should now work!"
echo "🔧 You can now deploy your insights agent with Kyverno policy sync enabled"
