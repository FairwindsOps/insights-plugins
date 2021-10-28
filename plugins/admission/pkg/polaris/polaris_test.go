package polaris

import (
	"context"
	"encoding/json"
	"testing"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/validator"
	"github.com/stretchr/testify/assert"
)

// TODO add tests for Polaris
const testPolarisObject = `
{
   "apiVersion": "apps/v1",
   "kind": "Deployment",
   "metadata": {
      "name": "nginx-deployment",
      "labels": {
         "app": "nginx"
      }
   },
   "spec": {
      "replicas": 3,
      "selector": {
         "matchLabels": {
            "app": "nginx"
         }
      },
      "template": {
         "metadata": {
            "labels": {
               "app": "nginx"
            }
         },
         "spec": {
            "containers": [
               {
                  "name": "nginx",
                  "image": "nginx:1.7.9",
                  "ports": [
                     {
                        "containerPort": 80
                     }
                  ],
                  "securityContext": {
                     "allowPrivilegeEscalation": false,
                     "privileged": false,
                     "readOnlyRootFilesystem": true,
                     "runAsNonRoot": true,
                     "capabilities": {
                        "drop": [
                           "ALL"
                        ]
                     }
                  }
               }
            ]
         }
      }
   }
}
`

func TestProcessPolaris(t *testing.T) {
	polarisConfig := polarisconfiguration.Configuration{}
	report, err := GetPolarisReport(context.TODO(), polarisConfig, []byte(testPolarisObject))
	assert.NoError(t, err)
	assert.Equal(t, "polaris", report.Report)
	var results validator.AuditData
	err = json.Unmarshal(report.Contents, &results)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results.Results))
}
