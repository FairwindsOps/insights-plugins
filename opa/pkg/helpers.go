package opa

import (
	"context"
	"errors"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
)

func getGroupKinds(targets []kubeTarget) []schema.GroupKind {
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

func getOutputArray(results rego.ResultSet) []interface{} {
	returnSet := make([]interface{}, 0)

	for _, result := range results {
		for _, pack := range result.Bindings["results"].(map[string]interface{}) {
			for _, outputArray := range pack.(map[string]interface{}) {
				for _, output := range outputArray.([]interface{}) {
					returnSet = append(returnSet, output)
				}
			}
		}
	}
	return returnSet
}

func getStringFromAST(astTerm *ast.Term) (string, error) {
	astString, ok := astTerm.Value.(ast.String)
	if !ok {
		return "", errors.New("Expected a string")
	}
	return strings.Trim(astString.String(), "\""), nil
}

func getKubernetesDataFunction(ctx context.Context) func(rego.BuiltinContext, *ast.Term, *ast.Term) (*ast.Term, error) {
	client := kube.GetKubeClient()
	return func(_ rego.BuiltinContext, groupAST, kindAST *ast.Term) (*ast.Term, error) {
		group, err1 := getStringFromAST(groupAST)
		kind, err2 := getStringFromAST(kindAST)
		if err1 != nil || err2 != nil {
			return nil, errors.New("the kubernetes function should be passed a group and kind as strings")
		}
		mapping, err := client.RestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
		if err != nil {
			return nil, err
		}
		gvr := mapping.Resource
		list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
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
	}

}
