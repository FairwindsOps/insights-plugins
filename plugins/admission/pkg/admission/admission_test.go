package admission

import (
	"context"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestIgnoreServicesAccount(t *testing.T) {
	v := NewValidator(&kubernetes.Clientset{}, models.InsightsConfig{})
	v.config = &models.Configuration{}
	req := admission.Request{}
	req.AdmissionRequest = v1.AdmissionRequest{}
	req.AdmissionRequest.UserInfo.Username = "test123"
	req.RequestKind = &metav1.GroupVersionKind{}
	resp := v.Handle(context.Background(), req)
	assert.Len(t, resp.Warnings, 0)
	v.config = &models.Configuration{
		IgnoreServicesAccount: []string{"test123"},
	}
	resp = v.Handle(context.Background(), req)
	assert.Equal(t, []string{"Service account test123 is being ignored by configuration"}, resp.Warnings)
}
