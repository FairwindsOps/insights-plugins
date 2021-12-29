package rego

import (
	"context"
	"errors"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/sirupsen/logrus"
)

type KubeDataFunction interface {
	GetData(context.Context, string, string) ([]interface{}, error)
}

type NilDataFunction struct{}

func (n NilDataFunction) GetData(ctx context.Context, group, kind string) ([]interface{}, error) {
	return nil, nil
}

func GetRegoQuery(body string, dataFn KubeDataFunction) *rego.Rego {
	return rego.New(
		rego.Query("results = data"),
		rego.Module("fairwinds", body),
		rego.Function2(
			&rego.Function{
				Name: "kubernetes",
				Decl: types.NewFunction(types.Args(types.S, types.S), types.A),
			},
			getDataFunction(dataFn.GetData)))
}

func RunRegoForItem(ctx context.Context, regoStr string, params map[string]interface{}, obj map[string]interface{}, dataFn KubeDataFunction) ([]interface{}, error) {
	r := GetRegoQuery(regoStr, dataFn)
	query, err := r.PrepareForEval(ctx)
	if err != nil {
		logrus.Errorf("Error while preparing rego query for evaluation: %v", err)
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
		logrus.Errorf("Error while evaluation query: %v", err)
		return nil, err
	}
	return getOutputArray(rs), nil
}

func getDataFunction(fn func(context.Context, string, string) ([]interface{}, error)) func(rego.BuiltinContext, *ast.Term, *ast.Term) (*ast.Term, error) {
	return func(rctx rego.BuiltinContext, groupAST, kindAST *ast.Term) (*ast.Term, error) {
		group, err1 := getStringFromAST(groupAST)
		kind, err2 := getStringFromAST(kindAST)

		if err1 != nil || err2 != nil {
			return nil, errors.New("the kubernetes function should be passed a group and kind as strings")
		}
		logrus.Infof("Getting Kubernetes data for %s/%s", group, kind)
		items, err := fn(rctx.Context, group, kind)
		if err != nil {
			logrus.Errorf("Error while getting data for %s/%s: %v", group, kind, err)
			return nil, err
		}
		itemValue, err := ast.InterfaceToValue(items)
		if err != nil {
			logrus.Errorf("Error while converting data for %s/%s: %v", group, kind, err)
			return nil, err
		}

		return ast.NewTerm(itemValue), nil
	}
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
