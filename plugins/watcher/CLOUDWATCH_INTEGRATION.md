# CloudWatch Integration for EKS Policy Violation Detection

## Overview

This PR adds comprehensive CloudWatch integration to the Kubernetes Event Watcher, enabling real-time detection of ValidatingAdmissionPolicy violations in EKS clusters through AWS CloudWatch logs.

## Key Features

### 🚀 **CloudWatch Integration**
- **Real-time Processing**: Processes EKS audit logs from CloudWatch in real-time
- **No Historical Data**: Only processes recent events (last 5 minutes) for performance
- **CloudWatch Filtering**: Uses filter patterns to reduce data transfer and processing overhead
- **Performance Optimized**: Configurable batch sizes, memory limits, and polling intervals
- **Multi-Environment Support**: Production, Staging, and Disaster-Recovery environments

### 🔐 **Security & Authentication**
- **IRSA Support**: Uses IAM Roles for Service Accounts for secure AWS access
- **Minimal Permissions**: CloudWatch Logs read-only permissions
- **No Hardcoded Credentials**: Leverages EKS native authentication
- **Environment Isolation**: Separate IAM roles for each environment

### 🎯 **ValidatingAdmissionPolicy Focus**
- **Primary Target**: Specifically designed to detect VAP violations from EKS audit logs
- **Efficient Detection**: Uses CloudWatch filter patterns to identify policy violations
- **Real-time Alerts**: Sends blocked policy violations to Fairwinds Insights immediately

## Implementation Details

### **New Components**

#### 1. CloudWatch Handler (`pkg/handlers/cloudwatch_handler.go`)
- Processes CloudWatch log events for ValidatingAdmissionPolicy violations
- Implements efficient batch processing with configurable limits
- Handles both filtered and unfiltered log events
- Creates policy violation events for blocked resources

#### 2. Enhanced Main Application (`cmd/insights-event-watcher/main.go`)
- Added CloudWatch-specific command line flags
- Comprehensive validation for CloudWatch configuration
- Support for dual log sources (local + CloudWatch)

#### 3. Updated Watcher (`pkg/watcher/watcher.go`)
- Integrated CloudWatch handler alongside existing audit log handler
- Proper lifecycle management for CloudWatch operations
- Enhanced error handling and logging

### **Helm Chart Integration**

#### Updated Templates
- **values.yaml**: Added CloudWatch configuration options
- **deployment.yaml**: CloudWatch command line arguments and environment variables
- **serviceaccount.yaml**: IRSA annotation support
- **rbac.yaml**: Refined permissions for ValidatingAdmissionPolicy focus

#### Configuration Example
```yaml
insights-event-watcher:
  enabled: true
  cloudwatch:
    enabled: true
    logGroupName: "/aws/eks/production-eks/cluster"
    region: "us-west-2"
    filterPattern: "{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }"
    batchSize: 100
    pollInterval: "30s"
    maxMemoryMB: 512
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/production-eks_cloudwatch_watcher"
```

## Usage

### **Command Line Interface**

#### Local Mode (Kind/Local Clusters)
```bash
./insights-event-watcher \
  --log-source=local \
  --audit-log-path=/var/log/kubernetes/kube-apiserver-audit.log
```

#### CloudWatch Mode (EKS Clusters)
```bash
./insights-event-watcher \
  --log-source=cloudwatch \
  --cloudwatch-log-group=/aws/eks/production-eks/cluster \
  --cloudwatch-region=us-west-2 \
  --cloudwatch-filter-pattern="{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }"
```

### **IAM Setup**

#### 1. Create IAM Role with IRSA Trust Policy
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/OIDC_PROVIDER_URL"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "OIDC_PROVIDER_URL:sub": "system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT_NAME"
        }
      }
    }
  ]
}
```

#### 2. Attach CloudWatch Logs Policy
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
        "logs:FilterLogEvents",
        "logs:GetLogEvents"
      ],
      "Resource": [
        "arn:aws:logs:*:*:log-group:/aws/eks/*/cluster",
        "arn:aws:logs:*:*:log-group:/aws/eks/*/cluster:*"
      ]
    }
  ]
}
```

## Performance Considerations

### **High-Volume Clusters**
- Increase batch size: `--cloudwatch-batch-size=500`
- Reduce poll interval: `--cloudwatch-poll-interval=15s`
- Increase memory limit: `--cloudwatch-max-memory=1024`

### **Low-Volume Clusters**
- Decrease batch size: `--cloudwatch-batch-size=50`
- Increase poll interval: `--cloudwatch-poll-interval=60s`
- Decrease memory limit: `--cloudwatch-max-memory=256`

## Testing

### **Local Testing**
1. Build the application: `go build ./cmd/insights-event-watcher`
2. Test with local mode: `./insights-event-watcher --log-source=local`
3. Verify CloudWatch mode validation: `./insights-event-watcher --log-source=cloudwatch --help`

### **EKS Testing**
1. Deploy with CloudWatch configuration
2. Create a ValidatingAdmissionPolicy that blocks resources
3. Attempt to create a violating resource
4. Verify policy violation is detected and sent to Insights

## Deployment

### **Terraform Infrastructure Setup**

#### **1. Apply Terraform Changes**
```bash
# For production
cd /Users/james/git/insights-terraform/production
terraform plan
terraform apply

# For staging
cd /Users/james/git/insights-terraform/staging
terraform plan
terraform apply

# For disaster-recovery
cd /Users/james/git/insights-terraform/disaster-recovery
terraform plan
terraform apply
```

#### **2. Environment-Specific IAM Roles**

| Environment | IAM Role Name | Log Group |
|-------------|---------------|-----------|
| **Production** | `production-eks_cloudwatch_watcher` | `/aws/eks/production-eks/cluster` |
| **Staging** | `staging-eks_cloudwatch_watcher` | `/aws/eks/staging-eks/cluster` |
| **Disaster-Recovery** | `production-dr-eks_cloudwatch_watcher` | `/aws/eks/production-dr-eks/cluster` |

#### **3. Service Account Annotations**

```yaml
# Production
eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/production-eks_cloudwatch_watcher"

# Staging
eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/staging-eks_cloudwatch_watcher"

# Disaster-Recovery
eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/production-dr-eks_cloudwatch_watcher"
```

### **Helm Chart Deployment**

#### **1. Update values.yaml**
```yaml
insights-event-watcher:
  cloudwatch:
    enabled: true
    logGroupName: "/aws/eks/your-cluster/cluster"  # Environment-specific
    region: "your-region"
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/your-environment-eks_cloudwatch_watcher"
```

#### **2. Deploy Updated Chart**
```bash
helm upgrade insights-agent ./charts/stable/insights-agent
```

## Migration Guide

### **From Local Mode to CloudWatch Mode**

1. **Deploy Terraform Infrastructure** (see above)
2. **Update values.yaml** with environment-specific configuration
3. **Deploy updated chart** with CloudWatch enabled

## Benefits

### **For EKS Clusters**
- **No Local Access Required**: No need for host path mounts or audit log access
- **Scalable**: Handles high-volume clusters with configurable performance tuning
- **Secure**: Uses IRSA for authentication, no hardcoded credentials
- **Real-time**: Processes violations as they occur, no delays

### **For Operations Teams**
- **Unified Monitoring**: Single tool for both local and EKS policy violation detection
- **Easy Configuration**: Simple Helm chart configuration
- **Comprehensive Logging**: Detailed logs for troubleshooting
- **Performance Monitoring**: Built-in resource limits and monitoring

## Future Enhancements

- **Multi-Region Support**: Support for cross-region log group monitoring
- **Custom Filter Patterns**: UI for creating and testing filter patterns
- **Metrics Integration**: CloudWatch metrics for monitoring watcher performance
- **Alert Integration**: Direct integration with AWS SNS for alerts

## Conclusion

This CloudWatch integration provides a robust, scalable solution for detecting ValidatingAdmissionPolicy violations in EKS clusters. It maintains the existing local mode functionality while adding powerful CloudWatch capabilities for production EKS environments.

The implementation follows established patterns from the insights-agent chart and provides comprehensive documentation and troubleshooting guides for easy adoption.
