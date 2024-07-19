package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/rego"
	"github.com/hashicorp/go-multierror"
)

var (
	defaultSeverity    = 0.5
	defaultTitle       = "Unknown title"
	defaultDescription = ""
	defaultRemediation = ""
	defaultCategory    = "Security"
	CLIKubeTargets     *[]KubeResourceTarget
)

var instanceGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customcheckinstances"}
var checkGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customchecks"}

func Run(ctx context.Context) ([]ActionItem, error) {
	jsonResponse, err := getInsightsChecks()
	if err != nil {
		return nil, err
	}

	logrus.Infof("Found %d checks and %d instances", len(jsonResponse.Checks), len(jsonResponse.Instances))

	ais, err := processAllChecks(ctx, jsonResponse.Instances, jsonResponse.Checks)
	if err != nil {
		return nil, err
	}
	return ais, nil
}

// processAllChecks runs the supplied slices of OPACustomCheck and
// CheckSetting and returns a slice of action items.
func processAllChecks(ctx context.Context, checkInstances []CheckSetting, checksAndLibs []OPACustomCheck) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var allErrs error = nil

	opaCustomChecks, opaCustomLibs := GetOPACustomChecksAndLibraries(checksAndLibs)
	for _, check := range opaCustomChecks {
		logrus.Debugf("Check %s is version %.1f", check.Name, check.Version)
		switch check.Version {
		case 1.0:
			for _, checkInstance := range checkInstances {
				if check.Name == checkInstance.CheckName {
					logrus.Debugf("Found instance %s to match check %s", checkInstance.AdditionalData.Name, check.Name)
					newItems, err := processCheck(ctx, check, checkInstance.GetCustomCheckInstance())
					if err != nil {
						allErrs = multierror.Append(allErrs, fmt.Errorf("error while processing check %s / instance %s: %v", check.Name, checkInstance.GetCustomCheckInstance().Name, multierror.Prefix(err, " ")))
						break
					}
					actionItems = append(actionItems, newItems...)
					break
				}
			}
		case 2.0:
			newItems, err := processCheckV2(ctx, check, opaCustomLibs)
			if err != nil {
				allErrs = multierror.Append(allErrs, fmt.Errorf("error while processing check %s: %v", check.Name, multierror.Prefix(err, " ")))
			}
			actionItems = append(actionItems, newItems...)
		default:
			allErrs = multierror.Append(allErrs, fmt.Errorf("CustomCheck %s is an unexpected version %.1f and will not be run", check.Name, check.Version))
		}
	}
	return actionItems, allErrs
}

// processCheck accepts a OPACustomCheck and CheckInstance, returning both
// action items and errors accumulated while processing the check.
func processCheck(ctx context.Context, check OPACustomCheck, checkInstance CustomCheckInstance) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var allErrs *multierror.Error = new(multierror.Error)
	allErrs.ErrorFormat = multierrorListFormatLimiterFunc
	for _, gk := range getGroupKinds(checkInstance.Spec.Targets) {
		newAI, err := processCheckTarget(ctx, check, checkInstance, gk)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}
		actionItems = append(actionItems, newAI...)
	}
	return actionItems, allErrs.ErrorOrNil()
}

// processCheckV2 accepts a OPACustomCheck, returning both action items and
// errors accumulated while processing the check.
func processCheckV2(ctx context.Context, check OPACustomCheck, opaCustomLibs []OPACustomLibrary) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var allErrs *multierror.Error = new(multierror.Error)
	allErrs.ErrorFormat = multierrorListFormatLimiterFunc
	for _, gr := range getGroupResources(*CLIKubeTargets) {
		newAI, err := processCheckTargetV2(ctx, check, gr, opaCustomLibs)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}
		actionItems = append(actionItems, newAI...)
	}
	return actionItems, allErrs.ErrorOrNil()
}

// processCheckTarget runs the specified OPACustomCheck and CheckInstance
// against all in-cluster objects of the specified schema.GroupKind, returning
// any action items.
func processCheckTarget(ctx context.Context, check OPACustomCheck, checkInstance CustomCheckInstance, gk schema.GroupKind) ([]ActionItem, error) {
	client := kube.GetKubeClient()
	actionItems := make([]ActionItem, 0)
	mapping, err := client.RestMapper.RESTMapping(gk)
	if err != nil {
		return actionItems, err
	}
	gvr := mapping.Resource
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range list.Items {
		newItems, err := ProcessCheckForItem(ctx, check, checkInstance, obj.Object, obj.GetName(), obj.GetKind(), obj.GetNamespace(), &rego.InsightsInfo{InsightsContext: "Agent", Cluster: os.Getenv("FAIRWINDS_CLUSTER")})
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

// processCheckTarget runs the specified OPACustomCheck against all in-cluster
// objects of the specified schema.GroupResource, returning any action items.
func processCheckTargetV2(ctx context.Context, check OPACustomCheck, gr schema.GroupResource, opaCustomLibs []OPACustomLibrary) ([]ActionItem, error) {
	client := kube.GetKubeClient()
	actionItems := make([]ActionItem, 0)
	gvr, err := client.RestMapper.ResourceFor(gr.WithVersion("")) // The empty APIVersion causes RESTMapper to provide one.
	if err != nil {
		return nil, fmt.Errorf("error getting GroupVersionResource from GroupResource %q: %v", gr, err)
	}
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing %q for check %s and target %s: %v", gvr, check.Name, gr, err)
	}
	logrus.Debugf("Listed %d %q objects for check %s, target %s", len(list.Items), gvr, check.Name, gr)
	for _, obj := range list.Items {
		newItems, err := ProcessCheckForItemV2(ctx, check, obj.Object, obj.GetName(), obj.GetKind(), obj.GetNamespace(), opaCustomLibs, &rego.InsightsInfo{InsightsContext: "Agent", Cluster: os.Getenv("FAIRWINDS_CLUSTER")})
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

// ProcessCheckForItem is a runRegoForItem() wrapper that uses the specified
// Kubernetes Kind/Namespace/Name to construct an action item.
func ProcessCheckForItem(ctx context.Context, check OPACustomCheck, instance CustomCheckInstance, obj map[string]any, resourceName, resourceKind, resourceNamespace string, insightsInfo *rego.InsightsInfo) ([]ActionItem, error) {
	results, err := runRegoForItem(ctx, check.Rego, instance.Spec.Parameters, obj, insightsInfo)
	if err != nil {
		return nil, fmt.Errorf("error while running rego for check %s on item %s/%s/%s: %v", check.Name, resourceKind, resourceNamespace, resourceName, err)
	}
	aiDetails := OutputFormat{}
	aiDetails.SetDefaults(check.GetOutputFormat(), instance.Spec.Output)
	newItems, err := processResults(resourceName, resourceKind, resourceNamespace, results, check.Name, aiDetails)
	return newItems, err
}

// ProcessCheckForItemV2 is a runRegoForItemV2() wrapper that uses the specified
// Kubernetes Kind/Namespace/Name to construct an action item.
func ProcessCheckForItemV2(ctx context.Context, check OPACustomCheck, obj map[string]any, resourceName, resourceKind, resourceNamespace string, opaCustomLibs []OPACustomLibrary, insightsInfo *rego.InsightsInfo) ([]ActionItem, error) {
	results, err := runRegoForItemV2(ctx, check.Rego, obj, opaCustomLibs, insightsInfo)
	if err != nil {
		return nil, fmt.Errorf("error while running rego for check %s on item %s/%s/%s: %v", check.Name, resourceKind, resourceNamespace, resourceName, err)
	}
	aiDetails := OutputFormat{}
	aiDetails.SetDefaults(check.GetOutputFormat())
	newItems, err := processResults(resourceName, resourceKind, resourceNamespace, results, check.Name, aiDetails)
	return newItems, err
}

// runRegoForItem accepts rego, Insights parameters, a Kube object, and
// InsightsInfo, running the rego policy with the Kubernetes object as input.
// The Insights Parameters and Insights Info struct are also made available to
// the executing rego policy (the latter via a function).
func runRegoForItem(ctx context.Context, body string, params map[string]any, obj map[string]any, insightsInfo *rego.InsightsInfo) ([]any, error) {
	client := kube.GetKubeClient()
	return rego.RunRegoForItem(ctx, body, params, obj, *client, insightsInfo)
}

// runRegoForItemV2 accepts rego, a Kube object, and InsightsInfo, running the
// rego policy with the Kubernetes object as input.  The Insights Info struct
// is also made available to the executing rego policy via a function.
func runRegoForItemV2(ctx context.Context, body string, obj map[string]any, opaCustomLibs []OPACustomLibrary, insightsInfo *rego.InsightsInfo) ([]any, error) {
	client := kube.GetKubeClient()
	return rego.RunRegoForItemV2(ctx, body, obj, *client, toOPACustomLibsMap(opaCustomLibs), insightsInfo)
}

func getInsightsChecks() (*clusterCheckModel, error) {
	url := os.Getenv("FAIRWINDS_INSIGHTS_HOST") + "/v0/organizations/" + os.Getenv("FAIRWINDS_ORG") + "/clusters/" + os.Getenv("FAIRWINDS_CLUSTER") +
		"/data/opa/customChecks"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("FAIRWINDS_TOKEN"))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to retrieve updated checks with a status code of : %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var jsonResponse clusterCheckModel
	err = json.Unmarshal(responseBody, &jsonResponse)
	if err != nil {
		return nil, err
	}
	return &jsonResponse, nil
}

func maybeGetStringField(m map[string]any, key string) (*string, error) {
	if m[key] == nil {
		return nil, nil
	}
	str, ok := m[key].(string)
	if !ok {
		return nil, errors.New(key + " was not a string")
	}
	return &str, nil
}

func maybeGetFloatField(m map[string]any, key string) (*float64, error) {
	if m[key] == nil {
		return nil, nil
	}
	n, ok := m[key].(json.Number)
	if !ok {
		return nil, errors.New(key + " was not a float")
	}
	f, err := n.Float64()
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func getDetailsFromMap(m map[string]any) (OutputFormat, error) {
	output := OutputFormat{}
	var err error
	output.Description, err = maybeGetStringField(m, "description")
	if err != nil {
		return output, err
	}
	output.Title, err = maybeGetStringField(m, "title")
	if err != nil {
		return output, err
	}
	output.Category, err = maybeGetStringField(m, "category")
	if err != nil {
		return output, err
	}
	output.Remediation, err = maybeGetStringField(m, "remediation")
	if err != nil {
		return output, err
	}
	output.Severity, err = maybeGetFloatField(m, "severity")
	if err != nil {
		return output, err
	}
	return output, nil
}

// processResults converts the provided rego results (output) and other
// metadata into action items.
func processResults(resourceName, resourceKind, resourceNamespace string, results []any, name string, details OutputFormat) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	for _, output := range results {
		strMethod, ok := output.(string)
		outputDetails := OutputFormat{}
		if ok {
			outputDetails.Description = &strMethod
		} else {
			mapMethod, ok := output.(map[string]any)
			if ok {
				var err error
				outputDetails, err = getDetailsFromMap(mapMethod)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("could not decipher output format of %+v %T", output, output)
			}
		}
		outputDetails.SetDefaults(details, OutputFormat{
			Severity:    &defaultSeverity,
			Title:       &defaultTitle,
			Remediation: &defaultRemediation,
			Category:    &defaultCategory,
			Description: &defaultDescription,
		})

		actionItems = append(actionItems, ActionItem{
			EventType:         name,
			ResourceNamespace: resourceNamespace,
			ResourceKind:      resourceKind,
			ResourceName:      resourceName,
			Title:             *outputDetails.Title,
			Description:       *outputDetails.Description,
			Remediation:       *outputDetails.Remediation,
			Severity:          *outputDetails.Severity,
			Category:          *outputDetails.Category,
		})
	}

	return actionItems, nil
}

// getGroupKinds accepts a slice of KubeKindTarget, returning all combinations
// of the contained Kubernetes APIGroups and Kinds as a slice of
// schema.GroupResource.
func getGroupKinds(targets []KubeTarget) []schema.GroupKind {
	kinds := make([]schema.GroupKind, 0)
	for _, target := range targets {
		for _, apiGroup := range target.APIGroups {
			for _, kind := range target.Kinds {
				kinds = append(kinds, schema.GroupKind{Group: apiGroup, Kind: kind})
			}
		}
	}
	return kinds
}

// getGroupResources accepts a slice of KubeResourceTarget, returning all
// combinations of the contained Kubernetes APIGroups and Resources as a slice
// of schema.GroupResource.
func getGroupResources(targets []KubeResourceTarget) []schema.GroupResource {
	GRs := make([]schema.GroupResource, 0)
	for _, target := range targets {
		for _, apiGroup := range target.APIGroups {
			for _, resource := range target.Resources {
				GRs = append(GRs, schema.GroupResource{Group: apiGroup, Resource: resource})
			}
		}
	}
	return GRs
}

// ProcessClIKubeResourceTargets processes command-line flag input and returns
// a slice of KubeResourceTarget.
func ProcessCLIKubeResourceTargets(CLIInputs []string) *[]KubeResourceTarget {
	if CLIInputs == nil {
		return nil
	}
	targets := make([]KubeResourceTarget, 0)
	for _, CLIInput := range CLIInputs {
		fields := strings.Split(CLIInput, "/")
		var target KubeResourceTarget
		target.APIGroups = append(target.APIGroups, strings.Split(fields[0], ",")...)
		target.Resources = append(target.Resources, strings.Split(fields[1], ",")...)
		if len(target.APIGroups) > 0 || len(target.Resources) > 0 {
			targets = append(targets, target)
		}
	}
	logrus.Debugf("Parsed command-line options %q into %d target resources: %#v", CLIInputs, len(targets), targets)
	return &targets
}

const maxMultiErrorsToShow = 3

// multierrorListFormatLimiterFunc is a github.com/hashicorp/go-multierror
// formatter that limits the number of errors output, to the `maxMultiErrorsToShow`
// constant.
// If debug logging is enabled, errors will NOT be limited.
// This helps provide a sampling of errors while avoiding unbounded output of errors in logs.
// This is based on the default multierror `ListFormatFunc()`.
// Ref: https://pkg.go.dev/github.com/hashicorp/go-multierror#ErrorFormatFunc
func multierrorListFormatLimiterFunc(es []error) string {
	if len(es) == 1 {
		return fmt.Sprintf("1 error occurred:\n\t* %s\n\n", es[0])
	}
	var extraHeader string
	var numToShow int = len(es)
	if len(es) > maxMultiErrorsToShow && logrus.GetLevel() != logrus.DebugLevel {
		numToShow = maxMultiErrorsToShow
		extraHeader = fmt.Sprintf(" (only %d are shown for brevity)", numToShow)
	}
	points := make([]string, numToShow)
	for i := 0; i < numToShow; i++ {
		points[i] = fmt.Sprintf("* %s", es[i])
	}
	return fmt.Sprintf(
		"%d errors occurred%s:\n\t%s\n\n",
		len(es), extraHeader, strings.Join(points, "\n\t"))
}

type OPACustomLibrary struct {
	Name string
	Rego string
}

func GetOPACustomChecksAndLibraries(customChecks []OPACustomCheck) ([]OPACustomCheck, []OPACustomLibrary) {
	checks := lo.Filter(customChecks, func(check OPACustomCheck, _ int) bool { return !check.IsLibrary })

	libChecks := lo.Filter(customChecks, func(check OPACustomCheck, _ int) bool { return check.IsLibrary })
	libs := lo.Map(libChecks, func(lib OPACustomCheck, _ int) OPACustomLibrary {
		return OPACustomLibrary{Name: lib.Name, Rego: lib.Rego}
	})

	return checks, libs
}

func toOPACustomLibsMap(opaCustomLibs []OPACustomLibrary) map[string]string {
	libs := map[string]string{}
	for _, lib := range opaCustomLibs {
		libs[lib.Name] = lib.Rego
	}
	return libs
}
