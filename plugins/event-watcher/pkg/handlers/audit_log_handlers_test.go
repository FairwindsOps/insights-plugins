package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const errorMessage = `Error from server: error when creating "deploy.yaml": admission webhook "validate.kyverno.svc-fail" denied the request: 

resource Deployment/default/nginx-deployment was blocked due to the following policies 

james-disallow-privileged-containers:
  check-privileged-james-1: 'validation error: Privileged containers are not allowed.
    rule check-privileged-james-1 failed at path /spec/containers/'
james-require-labels:
  check-required-labels-james-1: 'validation error: Required labels (app, version,
    environment) must be present. rule check-required-labels-james-1 failed at path
    /metadata/labels/environment/'
james-require-resource-limits:
  check-resource-limits-james-1: 'validation error: All containers must have resource	
    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'`

func TestAuditLogHandlerExtractPolicyName(t *testing.T) {
	result := ExtractPoliciesFromMessage(errorMessage)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "validation error: Privileged containers are not allowed. rule check-privileged-james-1 failed at path /spec/containers/", result["james-disallow-privileged-containers"]["check-privileged-james-1"])
	assert.Equal(t, "validation error: Required labels (app, version, environment) must be present. rule check-required-labels-james-1 failed at path /metadata/labels/environment/", result["james-require-labels"]["check-required-labels-james-1"])
	assert.Equal(t, "validation error: All containers must have resource limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/", result["james-require-resource-limits"]["check-resource-limits-james-1"])
}
