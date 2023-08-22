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

func TestIgnoreUsernames(t *testing.T) {
	v := NewValidator(&kubernetes.Clientset{}, models.InsightsConfig{})
	v.config = &models.Configuration{}
	req := admission.Request{}
	req.AdmissionRequest = v1.AdmissionRequest{}
	req.AdmissionRequest.UserInfo.Username = "test123"
	req.RequestKind = &metav1.GroupVersionKind{}
	resp := v.Handle(context.Background(), req)
	assert.Len(t, resp.Warnings, 0)
	v.iConfig = models.InsightsConfig{
		IgnoreUsernames: []string{"test123"},
	}
	resp = v.Handle(context.Background(), req)
	assert.Equal(t, []string{"[Fairwinds Insights] Insights admission controller is ignoring service account test123."}, resp.Warnings)
}
