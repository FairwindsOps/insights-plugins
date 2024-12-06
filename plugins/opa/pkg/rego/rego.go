package rego

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// InsightsInfo exposes Insights perspective, for consideration in rego
// policies.  FOr example, the context rego has been executed -
// Continuous Integration, Admission Controller, or the Insights Agent.
type InsightsInfo struct {
	InsightsContext  string
	Cluster          string
	Repository       string
	AdmissionRequest *admission.Request
}

type KubeDataFunction interface {
	GetData(context.Context, string, string) ([]interface{}, error)
}

type NilDataFunction struct{}

func (n NilDataFunction) GetData(ctx context.Context, group, kind string) ([]interface{}, error) {
	return nil, nil
}

func GetRegoQuery(body string, dataFn KubeDataFunction, opaCustomLibs map[string]string, insightsInfo *InsightsInfo) *rego.Rego {
	opts := []func(r *rego.Rego){
		rego.Query("results = data"),
		rego.Module("fairwinds", body),
		rego.Function2(
			&rego.Function{
				Name: "kubernetes",
				Decl: types.NewFunction(types.Args(types.S, types.S), types.A),
			},
			getDataFunction(dataFn.GetData),
		),
		rego.Function1(
			&rego.Function{
				Name: "insightsinfo",
				Decl: types.NewFunction(types.Args(types.S), types.A),
			},
			GetInsightsInfoFunction(insightsInfo),
		),
	}
	for name, regoDef := range opaCustomLibs {
		opts = append(opts, rego.Module(name, regoDef))
	}
	return rego.New(opts...)
}

func RunRegoForItem(ctx context.Context, regoStr string, params map[string]interface{}, obj map[string]interface{}, dataFn KubeDataFunction, insightsInfo *InsightsInfo) ([]interface{}, error) {
	r := GetRegoQuery(regoStr, dataFn, nil, insightsInfo)
	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	// TODO Find another way to get parameters in - Should they be a function or input?
	obj["parameters"] = params

	evaluatedInput := rego.EvalInput(obj)
	rs, err := query.Eval(ctx, evaluatedInput)
	if err != nil {
		return nil, err
	}
	return getOutputArray(rs), nil
}

// func RunRegoForItemV2 evaluates rego against a Kube object. IT replaces
// RunRegoForItemV() and supports v2 of Insights OPACustomChecks.
func RunRegoForItemV2(ctx context.Context, regoStr string, obj map[string]interface{}, dataFn KubeDataFunction, opaCustomLibs map[string]string, insightsInfo *InsightsInfo) ([]interface{}, error) {
	r := GetRegoQuery(regoStr, dataFn, opaCustomLibs, insightsInfo)
	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error while preparing rego query for evaluation: %v", err)
	}

	evaluatedInput := rego.EvalInput(obj)
	rs, err := query.Eval(ctx, evaluatedInput)
	if err != nil {
		return nil, fmt.Errorf("Error while evaluating query: %v", err)
	}
	return getOutputArray(rs), nil
}

func getDataFunction(fn func(context.Context, string, string) ([]interface{}, error)) func(rego.BuiltinContext, *ast.Term, *ast.Term) (*ast.Term, error) {
	return func(rctx rego.BuiltinContext, groupAST, kindAST *ast.Term) (*ast.Term, error) {
		group, err1 := getStringFromAST(groupAST)
		kind, err2 := getStringFromAST(kindAST)

		if err1 != nil || err2 != nil {
			return nil, rego.NewHaltError(errors.New("the kubernetes function should be passed a group and kind as strings"))
		}
		logrus.Infof("Getting Kubernetes data for %s/%s", group, kind)
		items, err := fn(rctx.Context, group, kind)
		if err != nil {
			return nil, rego.NewHaltError(fmt.Errorf("Error while getting data for %s/%s: %v", group, kind, err))
		}
		itemValue, err := ast.InterfaceToValue(items)
		if err != nil {
			return nil, rego.NewHaltError(fmt.Errorf("Error while converting data for %s/%s: %v", group, kind, err))
		}

		return ast.NewTerm(itemValue), nil
	}
}

// GetInsightsInfoFunction returns a function that is called from a rego
// policy, to provide Insights information to the policy depending on the
// function parameter.
func GetInsightsInfoFunction(insightsInfo *InsightsInfo) func(rego.BuiltinContext, *ast.Term) (*ast.Term, error) {
	return func(bc rego.BuiltinContext, inf *ast.Term) (*ast.Term, error) {
		reqInfo, err := getStringFromAST(inf)
		if err != nil {
			return nil, rego.NewHaltError(fmt.Errorf("unable to convert requested InsightsInfo to string: %w", err))
		}
		var retInfo any
		switch strings.ToLower(reqInfo) {
		case "context":
			retInfo = insightsInfo.InsightsContext
		case "cluster":
			retInfo = insightsInfo.Cluster
		case strings.ToLower("admissionRequest"):
			if insightsInfo.AdmissionRequest == nil {
				retInfo = nil // explicit set is required
			} else {
				retInfo = insightsInfo.AdmissionRequest.AdmissionRequest
			}
		default:
			return nil, rego.NewHaltError(fmt.Errorf("cannot return unknown Insights Info %q", reqInfo))
		}
		retInfoAsValue, err := ast.InterfaceToValue(retInfo)
		if err != nil {
			return nil, rego.NewHaltError(fmt.Errorf("unable to convert information %q to ast value: %w", retInfo, err))
		}
		return ast.NewTerm(retInfoAsValue), nil
	}
}

func getOutputArray(results rego.ResultSet) []interface{} {
	returnSet := make([]interface{}, 0)

	for _, result := range results {
		for _, pack := range result.Bindings["results"].(map[string]interface{}) {
			for _, outputArray := range pack.(map[string]interface{}) {
				if _, ok := outputArray.([]interface{}); ok {
					returnSet = append(returnSet, outputArray.([]interface{})...)
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
