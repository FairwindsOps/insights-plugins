package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditLogHandlerExtractPolicyName(t *testing.T) {
	handler := &AuditLogHandler{
		auditLogPath: "",
		eventChannel: nil,
	}

	result := handler.extractPolicyName("admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'james-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'james-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource\n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'")
	assert.Equal(t, "james-disallow-privileged-containers", result)
}
