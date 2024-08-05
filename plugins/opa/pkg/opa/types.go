package opa

import (
	"strings"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActionItem represents an action item from a report
type ActionItem struct {
	ResourceNamespace string
	ResourceKind      string
	ResourceName      string
	Title             string
	Description       string
	Remediation       string
	EventType         string
	Severity          float64
	Category          string
}

// CustomCheckInstance is an instance of a custom check
type CustomCheckInstance struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              CustomCheckInstanceSpec
}

// CustomCheckInstanceSpec is the body of an instance of a custom check
type CustomCheckInstanceSpec struct {
	Parameters      map[string]interface{}
	Targets         []KubeTarget
	Output          OutputFormat
	CustomCheckName string
}

// KubeTarget is a mapping of kinds and API groups
type KubeTarget struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string
}

// KubeResourceTarget is a combination of Resources and API groups, the
// cartesian product (all possible combinations) of which will be fetched from
// Kubernetes when applying V2 OPACustomChecks.
type KubeResourceTarget struct {
	APIGroups []string
	Resources []string
}

type OutputFormat struct {
	Title       *string
	Severity    *float64
	Remediation *string
	Category    *string
	Description *string
}

func (o *OutputFormat) SetDefaults(others ...OutputFormat) {
	for _, other := range others {
		if o.Title == nil {
			o.Title = other.Title
		}
		if o.Severity == nil {
			o.Severity = other.Severity
		}
		if o.Remediation == nil {
			o.Remediation = other.Remediation
		}
		if o.Category == nil {
			o.Category = other.Category
		}
		if o.Description == nil {
			o.Description = other.Description
		}
	}
}

type clusterCheckModel struct {
	Checks    []OPACustomCheck // contains both checks and libraries (IsLibrary)
	Instances []CheckSetting
}

type OPACustomCheck struct {
	Name                     string
	Version                  float32
	Rego                     string
	Title                    *string
	Severity                 *float64
	Remediation              *string
	Category                 *string
	AdditionalKubernetesData []string
	IsLibrary                bool
}

type CheckSetting struct {
	CheckName      string
	Targets        []string
	ClusterNames   []string
	AdditionalData struct {
		Name       string
		Output     OutputFormat
		Parameters map[string]interface{}
	}
}

func (instance CustomCheckInstance) MatchesTarget(apiGroup, kind string) bool {
	for _, target := range instance.Spec.Targets {
		for _, group := range target.APIGroups {
			for _, targetKind := range target.Kinds {
				if apiGroup == group && targetKind == kind {
					return true
				}
			}
		}
	}
	return false
}

func (supposedInstance CheckSetting) GetCustomCheckInstance() CustomCheckInstance {
	return CustomCheckInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: supposedInstance.AdditionalData.Name,
		},
		Spec: CustomCheckInstanceSpec{
			CustomCheckName: supposedInstance.CheckName,
			Output:          supposedInstance.AdditionalData.Output,
			Parameters:      supposedInstance.AdditionalData.Parameters,
			Targets: lo.Map(supposedInstance.Targets, func(s string, _ int) KubeTarget {
				splitValues := strings.Split(s, "/")
				return KubeTarget{
					APIGroups: []string{splitValues[0]},
					Kinds:     []string{splitValues[1]},
				}
			}),
		},
	}
}

func (supposedCheck OPACustomCheck) GetOutputFormat() OutputFormat {
	return OutputFormat{
		Title:       supposedCheck.Title,
		Severity:    supposedCheck.Severity,
		Remediation: supposedCheck.Remediation,
		Category:    supposedCheck.Category,
	}
}
