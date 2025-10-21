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

# Create the missing ClusterPolicy CRD with comprehensive schema
echo "🔧 Creating ClusterPolicy CRD with comprehensive schema..."
kubectl apply -f - << 'EOF'
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusterpolicies.kyverno.io
spec:
  group: kyverno.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              background:
                type: boolean
              validationFailureAction:
                type: string
              rules:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    match:
                      type: object
                      properties:
                        any:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        all:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        resources:
                          type: object
                        subjects:
                          type: array
                          items:
                            type: object
                    validate:
                      type: object
                      properties:
                        failureAction:
                          type: string
                        message:
                          type: string
                        pattern:
                          type: object
                          x-kubernetes-preserve-unknown-fields: true
                        anyPattern:
                          type: object
                          x-kubernetes-preserve-unknown-fields: true
                        deny:
                          type: object
                        foreach:
                          type: array
                          items:
                            type: object
                    mutate:
                      type: object
                      properties:
                        patchStrategicMerge:
                          type: object
                        patchesJson6902:
                          type: string
                        foreach:
                          type: array
                          items:
                            type: object
                    generate:
                      type: object
                      properties:
                        apiVersion:
                          type: string
                        kind:
                          type: string
                        name:
                          type: string
                        namespace:
                          type: string
                        data:
                          type: object
                        clone:
                          type: object
                    verifyImages:
                      type: array
                      items:
                        type: object
                        properties:
                          imageReferences:
                            type: array
                            items:
                              type: string
                          attestors:
                            type: array
                            items:
                              type: object
                          attestations:
                            type: array
                            items:
                              type: object
                    exclude:
                      type: object
                      properties:
                        any:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        all:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        resources:
                          type: object
                        subjects:
                          type: array
                          items:
                            type: object
  scope: Cluster
  names:
    plural: clusterpolicies
    singular: clusterpolicy
    kind: ClusterPolicy
    shortNames:
    - cpol
EOF

# Create the Policy CRD with comprehensive schema
echo "🔧 Creating Policy CRD with comprehensive schema..."
kubectl apply -f - << 'EOF'
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: policies.kyverno.io
spec:
  group: kyverno.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              background:
                type: boolean
              validationFailureAction:
                type: string
              rules:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    match:
                      type: object
                      properties:
                        any:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        all:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        resources:
                          type: object
                        subjects:
                          type: array
                          items:
                            type: object
                    validate:
                      type: object
                      properties:
                        failureAction:
                          type: string
                        message:
                          type: string
                        pattern:
                          type: object
                          x-kubernetes-preserve-unknown-fields: true
                        anyPattern:
                          type: object
                          x-kubernetes-preserve-unknown-fields: true
                        deny:
                          type: object
                        foreach:
                          type: array
                          items:
                            type: object
                    mutate:
                      type: object
                      properties:
                        patchStrategicMerge:
                          type: object
                        patchesJson6902:
                          type: string
                        foreach:
                          type: array
                          items:
                            type: object
                    generate:
                      type: object
                      properties:
                        apiVersion:
                          type: string
                        kind:
                          type: string
                        name:
                          type: string
                        namespace:
                          type: string
                        data:
                          type: object
                        clone:
                          type: object
                    verifyImages:
                      type: array
                      items:
                        type: object
                        properties:
                          imageReferences:
                            type: array
                            items:
                              type: string
                          attestors:
                            type: array
                            items:
                              type: object
                          attestations:
                            type: array
                            items:
                              type: object
                    exclude:
                      type: object
                      properties:
                        any:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        all:
                          type: array
                          items:
                            type: object
                            properties:
                              resources:
                                type: object
                                properties:
                                  kinds:
                                    type: array
                                    items:
                                      type: string
                                  names:
                                    type: array
                                    items:
                                      type: string
                                  namespaces:
                                    type: array
                                    items:
                                      type: string
                                  operations:
                                    type: array
                                    items:
                                      type: string
                                  selector:
                                    type: object
                              subjects:
                                type: array
                                items:
                                  type: object
                        resources:
                          type: object
                        subjects:
                          type: array
                          items:
                            type: object
  scope: Namespaced
  names:
    plural: policies
    singular: policy
    kind: Policy
    shortNames:
    - pol
EOF

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
