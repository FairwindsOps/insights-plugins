package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
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

const outputFile = "/output/opa.json"

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
	// TODO filter by namespace
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

		newItems, err := processCheck(ctx, checkObject, checkInstanceObject, restMapper, dynamicInterface)
		if err != nil {
			panic(err)
		}
		actionItems = append(actionItems, newItems...)
	}
	outputFormat := Output{ActionItems: actionItems}
	value, err := json.Marshal(outputFormat)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(outputFile, value, 0644)
	if err != nil {
		panic(err)
	}
}

func processCheck(ctx context.Context, check customCheck, checkInstance customCheckInstance, restMapper meta.RESTMapper, dynamicInterface dynamic.Interface) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	for _, target := range checkInstance.Spec.Targets {
		for _, apiGroup := range target.ApiGroups {
			for _, kind := range target.Kinds {
				fmt.Printf("Starting to process %s %s\n\n", apiGroup, kind)
				mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: apiGroup, Kind: kind})
				if err != nil {
					return nil, err
				}
				gvr := mapping.Resource
				list, err := dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, obj := range list.Items {
					query, err := rego.New(
						rego.Query("results = data"),
						rego.Module("fairwinds", check.Spec.Rego),
						rego.Function1(
							&rego.Function{
								Name: "kubernetes",
								Decl: types.NewFunction(types.Args(types.S), types.S),
							},
							func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {

								str, ok := a.Value.(ast.String)
								if !ok {
									return nil, nil
								}
								strValue := str.String()
								if len(strValue) > 0 && strValue[0] == '"' {
									strValue = strValue[1:]
								}
								if len(strValue) > 0 && strValue[len(strValue)-1] == '"' {
									strValue = strValue[:len(strValue)-1]
								}
								var apiGroup string
								for _, target := range check.Spec.AdditionalKubernetesData {
									fmt.Printf("Checking if %s matches %s", strValue, target.Kinds[0])
									if strValue == target.Kinds[0] {
										fmt.Printf("It does!")
										apiGroup = target.ApiGroups[0]
										break
									}
								}
								mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: apiGroup, Kind: strValue})
								if err != nil {
									return nil, err
								}
								gvr := mapping.Resource
								list, err := dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
								if err != nil {
									return nil, err
								}
								items := make([]interface{}, 0)
								for _, item := range list.Items {
									items = append(items, item.Object)
								}
								itemValue, err := ast.InterfaceToValue(items)
								if err != nil {
									return nil, err
								}
								return ast.NewTerm(itemValue), nil
							}),
					).PrepareForEval(ctx)
					if err != nil {
						return nil, err
					}
					obj.Object["parameters"] = checkInstance.Spec.Parameters
					// TODO Find another way to get parameters in - Should they be a function or input?
					// TODO Caching
					evaluatedInput := rego.EvalInput(obj.Object)
					results, err := query.Eval(ctx, evaluatedInput)
					if err != nil {
						return nil, err
					}
					newItems, err := processResults(obj, results, check, checkInstance)
					if err != nil {
						return nil, err
					}
					actionItems = append(actionItems, newItems...)
				}
			}
		}
	}
	return actionItems, nil
}

func processResults(resource unstructured.Unstructured, results rego.ResultSet, check customCheck, checkInstance customCheckInstance) ([]ActionItem, error) {
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
					category := checkOutput.Category
					if instanceOutput.Severity != nil {
						severity = instanceOutput.Severity
					}
					if instanceOutput.Title != nil {
						title = instanceOutput.Title
					}
					if instanceOutput.Remediation != nil {
						remediation = instanceOutput.Remediation
					}
					if instanceOutput.Category != nil {
						category = instanceOutput.Category
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
									return nil, err
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
					if category == nil {
						newCategory := "Reliability"
						category = &newCategory
					}

					actionItems = append(actionItems, ActionItem{
						ResourceNamespace: resource.GetNamespace(),
						ResourceKind:      resource.GetKind(),
						ResourceName:      resource.GetName(),
						Title:             *title,
						Description:       description,
						Remediation:       *remediation,
						Severity:          *severity,
						Category:          *category,
					})
				}
			}
		}
	}
	return actionItems, nil
}
