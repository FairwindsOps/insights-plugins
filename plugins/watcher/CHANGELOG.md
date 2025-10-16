# Changelog

## 0.1.0 (Unreleased)

### Added
- **CloudWatch Integration**: Real-time processing of EKS audit logs from AWS CloudWatch
- **Dual Log Sources**: Support for both local audit logs (Kind/local) and CloudWatch logs (EKS)
- **ValidatingAdmissionPolicy Focus**: Primary focus on ValidatingAdmissionPolicy violations from EKS
- **IRSA Support**: IAM Roles for Service Accounts for secure AWS access
- **Performance Optimization**: Configurable batch sizes, memory limits, and CloudWatch filtering
- **Real-time Processing**: Processes only recent events (last 5 minutes), no historical data
- **CloudWatch Filter Patterns**: Efficient filtering at CloudWatch level to reduce data transfer
- **Enhanced Command Line Options**: New CloudWatch-specific configuration options
- **Insights-Agent Integration**: Full integration with insights-agent Helm chart
- **Comprehensive Documentation**: Updated README with CloudWatch setup and troubleshooting

### Changed
- **RBAC Permissions**: Refined to focus on ValidatingAdmissionPolicy resources
- **Resource Limits**: Increased default memory limits for CloudWatch processing
- **Service Account**: Added support for IRSA annotations
- **Deployment Templates**: Updated to support CloudWatch configuration

### Technical Details
- **AWS SDK v2**: Upgraded to AWS SDK v2 for CloudWatch integration
- **CloudWatch Handler**: New handler for processing CloudWatch log events
- **Event Processing**: Enhanced event processing for ValidatingAdmissionPolicy violations
- **Error Handling**: Improved error handling and logging for CloudWatch operations

## 0.0.1
* Initial release
