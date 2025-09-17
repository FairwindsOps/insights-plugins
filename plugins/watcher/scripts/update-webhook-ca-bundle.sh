#!/bin/bash

# Script to update webhook configuration with CA bundle
# This script extracts the CA certificate from the TLS secret and updates the webhook configuration

set -e

NAMESPACE="${NAMESPACE:-insights-agent}"
SECRET_NAME="${SECRET_NAME:-insights-event-watcher-webhook-tls}"
WEBHOOK_CONFIG="${WEBHOOK_CONFIG:-vap-audit-mutator}"

echo "Updating webhook CA bundle..."

# Extract CA certificate from the secret
CA_BUNDLE=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.ca\.crt}')

if [ -z "$CA_BUNDLE" ]; then
    echo "Warning: No ca.crt found in secret $SECRET_NAME. Using tls.crt instead."
    CA_BUNDLE=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.tls\.crt}')
fi

if [ -z "$CA_BUNDLE" ]; then
    echo "Error: No certificate found in secret $SECRET_NAME"
    exit 1
fi

# Update the webhook configuration
kubectl patch mutatingwebhookconfiguration "$WEBHOOK_CONFIG" --type='json' -p='[{"op": "replace", "path": "/webhooks/0/clientConfig/caBundle", "value": "'"$CA_BUNDLE"'"}]'

echo "Successfully updated webhook CA bundle for $WEBHOOK_CONFIG"
