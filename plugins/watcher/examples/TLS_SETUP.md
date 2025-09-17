# TLS Configuration for Insights Event Watcher Webhook

This document describes how to configure TLS certificates for the Insights Event Watcher webhook server, following the same pattern as the admission plugin.

## Overview

The webhook server uses controller-runtime's built-in webhook server with TLS support. TLS is handled at the Kubernetes level through:

1. **Certificate Management**: Using cert-manager or manual certificate generation
2. **Kubernetes Secrets**: Storing certificates in Kubernetes secrets
3. **Volume Mounts**: Mounting certificates into the pod
4. **Webhook Configuration**: Configuring the webhook to use HTTPS

## Architecture

```
┌─────────────────┐    HTTPS     ┌──────────────────┐
│   Kubernetes    │─────────────▶│  Webhook Server  │
│   API Server    │              │  (Port 8443)     │
└─────────────────┘              └──────────────────┘
                                         │
                                         ▼
                                ┌──────────────────┐
                                │  TLS Certificates│
                                │  /opt/cert/      │
                                │  - tls.crt       │
                                │  - tls.key       │
                                └──────────────────┘
```

## Certificate Management Options

### Option 1: cert-manager (Recommended)

1. **Install cert-manager** (if not already installed):
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   ```

2. **Create ClusterIssuer**:
   ```bash
   kubectl apply -f webhook-issuer.yaml
   ```

3. **Create Certificate**:
   ```bash
   kubectl apply -f webhook-certificate.yaml
   ```

4. **Deploy the webhook**:
   ```bash
   kubectl apply -f webhook-deployment.yaml
   ```

5. **Update webhook configuration with CA bundle**:
   ```bash
   ./scripts/update-webhook-ca-bundle.sh
   ```

### Option 2: Manual Certificate Generation

1. **Generate certificates**:
   ```bash
   ./scripts/generate-webhook-certs.sh
   ```

2. **Apply the TLS secret**:
   ```bash
   kubectl apply -f /tmp/webhook-certs/webhook-tls-secret.yaml
   ```

3. **Deploy the webhook**:
   ```bash
   kubectl apply -f webhook-deployment.yaml
   ```

4. **Update webhook configuration with CA bundle**:
   ```bash
   ./scripts/update-webhook-ca-bundle.sh
   ```

## Configuration Details

### Webhook Server Configuration

The webhook server is configured with:
- **Port**: 8443 (HTTPS)
- **Certificate Directory**: `/opt/cert`
- **Certificate File**: `tls.crt`
- **Private Key File**: `tls.key`
- **Health Check Port**: 8081 (HTTP)

### Kubernetes Resources

#### Deployment
- Mounts TLS certificates from Kubernetes secret
- Configures health checks on port 8081
- Uses controller-runtime webhook server

#### Service
- Exposes webhook on port 443 (maps to container port 8443)
- Uses ClusterIP for internal communication

#### MutatingWebhookConfiguration
- Configured to use HTTPS (port 8443)
- Includes CA bundle for certificate validation
- Targets ValidatingAdmissionPolicyBinding resources

## Testing

### Test the HTTPS Webhook

Run the comprehensive test script:
```bash
./scripts/test-https-webhook.sh
```

This script will:
1. Create test ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding
2. Verify that the webhook mutates the binding (adds Audit action)
3. Test health endpoints
4. Clean up test resources

### Manual Testing

1. **Check webhook pod status**:
   ```bash
   kubectl get pods -n insights-agent -l app=insights-event-watcher
   ```

2. **Check webhook logs**:
   ```bash
   kubectl logs -n insights-agent -l app=insights-event-watcher
   ```

3. **Test health endpoints**:
   ```bash
   kubectl port-forward -n insights-agent svc/insights-agent-insights-event-watcher 8081:443
   curl http://localhost:8081/healthz
   curl http://localhost:8081/readyz
   ```

4. **Create test ValidatingAdmissionPolicyBinding**:
   ```bash
   kubectl apply -f test-vap-deny-only.yaml
   ```

5. **Verify mutation**:
   ```bash
   kubectl get validatingadmissionpolicybinding test-deny-only-binding -o yaml
   ```

## Troubleshooting

### Common Issues

1. **Certificate not found**:
   - Check if the TLS secret exists: `kubectl get secret -n insights-agent`
   - Verify certificate files in the secret: `kubectl describe secret insights-event-watcher-webhook-tls -n insights-agent`

2. **Webhook not responding**:
   - Check pod logs: `kubectl logs -n insights-agent -l app=insights-event-watcher`
   - Verify service endpoints: `kubectl get endpoints -n insights-agent`
   - Check webhook configuration: `kubectl get mutatingwebhookconfiguration vap-audit-mutator -o yaml`

3. **CA bundle not set**:
   - Run the CA bundle update script: `./scripts/update-webhook-ca-bundle.sh`
   - Manually update the webhook configuration

4. **TLS handshake errors**:
   - Verify certificate validity: `kubectl exec -n insights-agent <pod> -- openssl x509 -in /opt/cert/tls.crt -text -noout`
   - Check certificate expiration: `kubectl exec -n insights-agent <pod> -- openssl x509 -in /opt/cert/tls.crt -noout -dates`

### Logs to Check

- **Webhook server logs**: Look for TLS-related messages
- **cert-manager logs**: Check certificate issuance status
- **Kubernetes API server logs**: Look for webhook call failures

## Security Considerations

1. **Certificate Rotation**: cert-manager automatically handles certificate rotation
2. **RBAC**: The webhook has minimal required permissions
3. **Network Policies**: Consider restricting webhook access with network policies
4. **Certificate Validation**: The webhook validates client certificates when CA is provided

## Migration from HTTP to HTTPS

If you're migrating from HTTP to HTTPS:

1. **Update webhook configuration** to use port 8443
2. **Deploy with TLS certificates**
3. **Update CA bundle** in webhook configuration
4. **Test thoroughly** before removing HTTP configuration
5. **Monitor logs** for any TLS-related issues

## Files Reference

- `webhook-deployment.yaml`: Complete deployment with TLS configuration
- `webhook-certificate.yaml`: cert-manager Certificate resource
- `webhook-issuer.yaml`: cert-manager ClusterIssuer resource
- `vap-mutating-webhook.yaml`: Updated webhook configuration with HTTPS
- `generate-webhook-certs.sh`: Manual certificate generation script
- `update-webhook-ca-bundle.sh`: CA bundle update script
- `test-https-webhook.sh`: Comprehensive test script
