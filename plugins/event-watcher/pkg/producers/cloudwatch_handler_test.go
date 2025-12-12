package producers

import (
	"encoding/json"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
	"github.com/stretchr/testify/assert"
)

const cloudWatchKyvernoBlock = `
{
    "kind": "Event",
    "apiVersion": "audit.k8s.io/v1",
    "level": "Request",
    "auditID": "5c0888f1-bdd0-4681-9aba-5b734c267df2",
    "stage": "ResponseComplete",
    "requestURI": "/apis/admissionregistration.k8s.io/v1/validatingadmissionpolicybindings?limit=500&resourceVersion=0",
    "verb": "list",
    "user": {
        "username": "system:apiserver",
        "uid": "7adf9ab0-3065-4682-bd5b-bd28d770c502",
        "groups": [
            "system:authenticated",
            "system:masters"
        ]
    },
    "sourceIPs": [
        "::1"
    ],
    "userAgent": "kube-apiserver/v1.33.5 (linux/arm64) kubernetes/0862ded",
    "objectRef": {
        "resource": "Deployment",
        "namespace": "default",
        "name": "nginx-deployment",
        "uid": "5c0888f1-bdd0-4681-9aba-5b734c267df2",
        "apiGroup": "apps",
        "apiVersion": "v1",
        "resourceVersion": "1234567890",
        "subResource": "status"
    },
    "responseStatus": {
        "metadata": {},
        "code": 400,
        "status": "Failure",
        "message": "Error from server: error when creating \"deploy.yaml\": admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'\n\njames-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'\njames-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource  \n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
        "reason": "BadRequest"
    },
    "requestReceivedTimestamp": "2025-10-23T10:23:37.368934Z",
    "stageTimestamp": "2025-10-23T10:23:37.369146Z",
    "annotations": {
        "authorization.k8s.io/decision": "deny",
        "authorization.k8s.io/reason": "james-disallow-privileged-containers: check-privileged-james-1: 'validation error: Privileged containers are not allowed. rule check-privileged-james-1 failed at path /spec/containers/'"
    }
}
`

func TestCloudWatchHandlerCreateBlockedWatchedEventFromPolicyViolationEvent(t *testing.T) {
	insightsConfig := models.InsightsConfig{
		Hostname:     "test-hostname",
		Organization: "test-organization",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}
	cloudwatchConfig := models.CloudWatchConfig{
		LogGroupName:  "test-log-group-name",
		Region:        "test-region",
		FilterPattern: "test-filter-pattern",
		BatchSize:     100,
		PollInterval:  "1s",
		MaxMemoryMB:   100,
	}
	_, err := NewCloudWatchHandler(insightsConfig, cloudwatchConfig, make(chan *models.WatchedEvent))
	if err != nil {
		t.Fatalf("Failed to create cloudwatch handler: %v", err)
	}

	auditEvent := models.AuditEvent{}
	err = json.Unmarshal([]byte(cloudWatchKyvernoBlock), &auditEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal cloud watch kyverno block: %v", err)
	}

	policyViolationEvent := utils.CreateBlockedPolicyViolationEvent(auditEvent)

	assert.Equal(t, "Deployment", policyViolationEvent.ResourceType)
	assert.Equal(t, "default", policyViolationEvent.Namespace)
	assert.Equal(t, "pol-violation-Deployment-nginx-deployment-5c0888f1-bdd0-4681-9aba-5b734c267df2", policyViolationEvent.Name)
	assert.Equal(t, "Failure", policyViolationEvent.Action)
	assert.Equal(t, "5c0888f1-bdd0-4681-9aba-5b734c267df2", policyViolationEvent.AuditID)

	utils.CreateBlockedWatchedEventFromPolicyViolationEvent(policyViolationEvent, make(chan *models.WatchedEvent))
}
