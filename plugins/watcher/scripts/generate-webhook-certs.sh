#!/bin/bash

# Script to generate TLS certificates for the webhook server
# This script creates a self-signed certificate for development/testing purposes

set -e

# Configuration
CERT_DIR="${CERT_DIR:-/tmp/webhook-certs}"
NAMESPACE="${NAMESPACE:-insights-agent}"
SERVICE_NAME="${SERVICE_NAME:-insights-agent-insights-event-watcher}"
CERT_NAME="${CERT_NAME:-webhook-cert}"
KEY_NAME="${KEY_NAME:-webhook-key}"
CA_NAME="${CA_NAME:-webhook-ca}"

# Create certificate directory
mkdir -p "$CERT_DIR"

# Generate CA private key
openssl genrsa -out "$CERT_DIR/$CA_NAME.key" 2048

# Generate CA certificate
openssl req -new -x509 -days 365 -key "$CERT_DIR/$CA_NAME.key" -out "$CERT_DIR/$CA_NAME.crt" -subj "/CN=insights-webhook-ca"

# Generate server private key
openssl genrsa -out "$CERT_DIR/$KEY_NAME.key" 2048

# Generate server certificate signing request
openssl req -new -key "$CERT_DIR/$KEY_NAME.key" -out "$CERT_DIR/$CERT_NAME.csr" -subj "/CN=$SERVICE_NAME"

# Generate server certificate
openssl x509 -req -in "$CERT_DIR/$CERT_NAME.csr" -CA "$CERT_DIR/$CA_NAME.crt" -CAkey "$CERT_DIR/$CA_NAME.key" -CAcreateserial -out "$CERT_DIR/$CERT_NAME.crt" -days 365 -extensions v3_req -extfile <(
cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = $SERVICE_NAME

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $SERVICE_NAME
DNS.2 = $SERVICE_NAME.$NAMESPACE.svc
DNS.3 = $SERVICE_NAME.$NAMESPACE.svc.cluster.local
DNS.4 = localhost
IP.1 = 127.0.0.1
EOF
)

# Create Kubernetes secret
kubectl create secret tls insights-event-watcher-webhook-tls \
  --cert="$CERT_DIR/$CERT_NAME.crt" \
  --key="$CERT_DIR/$KEY_NAME.key" \
  --namespace="$NAMESPACE" \
  --dry-run=client -o yaml > "$CERT_DIR/webhook-tls-secret.yaml"

echo "Certificates generated successfully!"
echo "Certificate directory: $CERT_DIR"
echo "Files created:"
echo "  - $CA_NAME.crt (CA certificate)"
echo "  - $CA_NAME.key (CA private key)"
echo "  - $CERT_NAME.crt (Server certificate)"
echo "  - $KEY_NAME.key (Server private key)"
echo "  - webhook-tls-secret.yaml (Kubernetes secret manifest)"
echo ""
echo "To apply the secret to your cluster:"
echo "  kubectl apply -f $CERT_DIR/webhook-tls-secret.yaml"
echo ""
echo "To use the certificates with the webhook server:"
echo "  --enable-tls=true"
echo "  --tls-cert-file=$CERT_DIR/$CERT_NAME.crt"
echo "  --tls-private-key-file=$CERT_DIR/$KEY_NAME.key"
echo "  --tls-ca-file=$CERT_DIR/$CA_NAME.crt"
