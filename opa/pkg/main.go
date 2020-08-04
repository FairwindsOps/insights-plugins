package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/open-policy-agent/opa/rego"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	ctx := context.TODO()
	instanceGvr := schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1", Resource: "customcheckinstances"}
	checkGvr := schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1", Resource: "customchecks"}
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}
	dynamicInterface, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		panic(err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	checkInstances, err := dynamicInterface.Resource(instanceGvr).Namespace("").List(ctx, metav1.ListOptions{})

	if err != nil {
		panic(err)
	}
	actionItems := make([]ActionItem, 0)

	for _, checkInstance := range checkInstances.Items {
		var checkInstanceObject customCheckInstance
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(checkInstance.Object, &checkInstanceObject)
		if err != nil {
			panic(err)
		}
		check, err := dynamicInterface.Resource(checkGvr).Namespace(checkInstanceObject.Namespace).Get(ctx, checkInstanceObject.Spec.CustomCheckName, metav1.GetOptions{})
		if err != nil {
			panic(err)
		}
		var checkObject customCheck
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(check.Object, &checkObject)
		if err != nil {
			panic(err)
		}

		actionItems = append(actionItems, processCheck(ctx, checkObject, checkInstanceObject, restMapper, dynamicInterface)...)
	}
	value, err := json.Marshal(actionItems)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(value))
}

func processCheck(ctx context.Context, check customCheck, checkInstance customCheckInstance, restMapper meta.RESTMapper, dynamicInterface dynamic.Interface) []ActionItem {
	actionItems := make([]ActionItem, 0)

	for _, target := range checkInstance.Spec.Targets {
		for _, apiGroup := range target.ApiGroups {
			for _, kind := range target.Kinds {
				fmt.Printf("Starting to process %s %s\n\n", apiGroup, kind)
				mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: apiGroup, Kind: kind})
				if err != nil {
					panic(err)
				}
				gvr := mapping.Resource
				list, err := dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					panic(err)
				}
				query, err := rego.New(
					rego.Query("results = data"),
					rego.Module("fairwinds", check.Spec.Rego),
				).PrepareForEval(ctx)
				if err != nil {
					panic(err)
				}
				for _, obj := range list.Items {
					obj.Object["parameters"] = checkInstance.Spec.Parameters
					evaluatedInput := rego.EvalInput(obj.Object)
					results, err := query.Eval(ctx, evaluatedInput)
					if err != nil {
						panic(err)
					}
					actionItems = append(actionItems, processResults(obj, results, check, checkInstance)...)
				}
			}
		}
	}
	return actionItems
}

func processResults(resource unstructured.Unstructured, results rego.ResultSet, check customCheck, checkInstance customCheckInstance) []ActionItem {
	actionItems := make([]ActionItem, 0)
	instanceOutput := checkInstance.Spec.Output
	checkOutput := check.Spec.Output
	for _, result := range results {
		for _, pack := range result.Bindings["results"].(map[string]interface{}) {
			for _, outputArray := range pack.(map[string]interface{}) {
				for _, output := range outputArray.([]interface{}) {
					severity := instanceOutput.Severity
					title := checkOutput.Title
					remediation := checkOutput.Remediation
					if instanceOutput.Severity != nil {
						severity = instanceOutput.Severity
					}
					if instanceOutput.Title != nil {
						title = instanceOutput.Title
					}
					if instanceOutput.Remediation != nil {
						remediation = instanceOutput.Remediation
					}
					strMethod, ok := output.(string)
					var description string
					if ok {
						description = fmt.Sprintf("String: %s", strMethod)
					} else {
						mapMethod, ok := output.(map[string]interface{})
						if ok {
							description = fmt.Sprintf("Map: Desc: %v", mapMethod["description"])
							if mapMethod["severity"] != nil {
								severityFloat, err := strconv.ParseFloat(mapMethod["severity"].(string), 64)
								if err != nil {
									panic(err)
								}
								severity = &severityFloat
							}
							if mapMethod["title"] != nil {
								titleString := mapMethod["title"].(string)
								title = &titleString
							}

							if mapMethod["remediation"] != nil {
								remediationString := mapMethod["remediation"].(string)
								remediation = &remediationString

							}
						} else {
							description = fmt.Sprintf("Could not decipher output format of %+v %T", output, output)
						}
					}
					if severity == nil {
						var severityFloat float64 = 0.0
						severity = &severityFloat
					}
					if title == nil {
						newTitle := "Unknown Title"
						title = &newTitle
					}
					if remediation == nil {
						newRemediation := ""
						remediation = &newRemediation
					}
					//TODO add to CRD
					category := "Efficiency"

					actionItems = append(actionItems, ActionItem{
						ResourceNamespace: resource.GetNamespace(),
						ResourceKind:      resource.GetKind(),
						ResourceName:      resource.GetName(),
						Title:             *title,
						Description:       description,
						Remediation:       *remediation,
						Severity:          *severity,
						Category:          category,
					})
				}
			}
		}
	}
	return actionItems
}
