package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestFormatNodePool(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodePool",
		"metadata": map[string]any{
			"name": "default",
			"uid":  "uid-1",
			"labels": map[string]any{
				"team": "platform",
			},
		},
		"spec": map[string]any{
			"weight": int64(10),
			"limits": map[string]any{
				"cpu":    "1000",
				"memory": float64(64), // quantity sometimes lands as number in unstructured
			},
			"disruption": map[string]any{
				"consolidationPolicy": "WhenEmptyOrUnderutilized",
				"consolidateAfter":    "1m",
				"budgets": []any{
					map[string]any{"nodes": "10%"},
				},
			},
			"template": map[string]any{
				"spec": map[string]any{
					"nodeClassRef": map[string]any{
						"group": "karpenter.k8s.aws",
						"kind":  "EC2NodeClass",
						"name":  "default",
					},
					"requirements": []any{
						map[string]any{
							"key":      "karpenter.sh/capacity-type",
							"operator": "In",
							"values":   []any{"spot", "on-demand"},
						},
					},
				},
			},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}}
	item.SetName("default")
	item.SetUID("uid-1")
	item.SetLabels(map[string]string{"team": "platform"})
	item.SetAPIVersion("karpenter.sh/v1")

	got := formatNodePool(item)
	require.Equal(t, KindNodePool, got.Kind)
	require.Equal(t, "default", got.Name)
	require.Equal(t, "uid-1", got.UID)
	require.NotNil(t, got.Weight)
	require.Equal(t, int32(10), *got.Weight)
	require.Equal(t, "1000", got.Limits["cpu"])
	require.Equal(t, "64", got.Limits["memory"])
	require.NotNil(t, got.NodeClassRef)
	require.Equal(t, "default", got.NodeClassRef.Name)
	require.Equal(t, "EC2NodeClass", got.NodeClassRef.Kind)
	require.Len(t, got.Requirements, 1)
	require.Equal(t, "karpenter.sh/capacity-type", got.Requirements[0].Key)
	require.NotNil(t, got.Disruption)
	require.Equal(t, "WhenEmptyOrUnderutilized", got.Disruption.ConsolidationPolicy)
	require.Equal(t, "10%", got.Disruption.Budgets[0].Nodes)
	require.Len(t, got.Conditions, 1)
}

func TestFormatNodeClaim(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodeClaim",
		"metadata": map[string]any{
			"name": "claim-1",
			"uid":  "uid-2",
			"labels": map[string]any{
				"karpenter.sh/nodepool": "default",
			},
		},
		"spec": map[string]any{
			"nodeClassRef": map[string]any{
				"group": "karpenter.k8s.aws",
				"kind":  "EC2NodeClass",
				"name":  "default",
			},
		},
		"status": map[string]any{
			"providerID": "aws:///us-east-1a/i-abc",
			"nodeName":   "ip-10-0-1-2.ec2.internal",
		},
	}}
	item.SetName("claim-1")
	item.SetUID("uid-2")
	item.SetLabels(map[string]string{karpenterNodePoolLabelKey: "default"})
	item.SetAPIVersion("karpenter.sh/v1")

	got := formatNodeClaim(item)
	require.Equal(t, KindNodeClaim, got.Kind)
	require.Equal(t, "default", got.NodePool)
	require.Equal(t, "aws:///us-east-1a/i-abc", got.ProviderID)
	require.Equal(t, "ip-10-0-1-2.ec2.internal", got.NodeName)
	require.NotNil(t, got.NodeClassRef)
	require.Equal(t, "default", got.NodeClassRef.Name)
}

func TestFormatEC2NodeClass(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
			"uid":  "uid-3",
		},
		"spec": map[string]any{
			"role":      "KarpenterNodeRole",
			"amiFamily": "AL2023",
			"tags": map[string]any{
				"team": "platform",
			},
			"subnetSelectorTerms": []any{
				map[string]any{
					"tags": map[string]any{"karpenter.sh/discovery": "cluster"},
				},
			},
			"securityGroupSelectorTerms": []any{
				map[string]any{"id": "sg-123"},
			},
			"amiSelectorTerms": []any{
				map[string]any{"alias": "al2023@latest"},
			},
		},
	}}
	item.SetName("default")
	item.SetUID("uid-3")
	item.SetAPIVersion("karpenter.k8s.aws/v1")

	got := formatEC2NodeClass(item)
	require.Equal(t, KindEC2NodeClass, got.Kind)
	require.Equal(t, "default", got.Name)
	require.Equal(t, "KarpenterNodeRole", got.Role)
	require.Equal(t, "AL2023", got.AMIFamily)
	require.Equal(t, "platform", got.Tags["team"])
	require.Len(t, got.SubnetSelectorTerms, 1)
	require.Equal(t, "cluster", got.SubnetSelectorTerms[0].Tags["karpenter.sh/discovery"])
	require.Equal(t, "sg-123", got.SecurityGroupSelectorTerms[0].ID)
	require.Equal(t, "al2023@latest", got.AMISelectorTerms[0].Alias)
}

func TestFormatNodePoolEmptySpec(t *testing.T) {
	item := unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{"name": "empty"},
		},
	}
	item.SetName("empty")
	got := formatNodePool(item)
	require.Equal(t, "empty", got.Name)
	require.Nil(t, got.Disruption)
	require.Nil(t, got.NodeClassRef)
	require.Nil(t, got.Requirements)
}

func TestKarpenterEmptyArraysJSONContract(t *testing.T) {
	report := ClusterWorkloadReport{
		NodePools:      []NodePool{},
		NodeClaims:     []NodeClaim{},
		EC2NodeClasses: []EC2NodeClass{},
	}
	raw, err := json.Marshal(report)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Contains(t, decoded, "NodePools")
	require.Contains(t, decoded, "NodeClaims")
	require.Contains(t, decoded, "EC2NodeClasses")
	require.IsType(t, []any{}, decoded["NodePools"])
	require.Empty(t, decoded["NodePools"])
	require.Empty(t, decoded["NodeClaims"])
	require.Empty(t, decoded["EC2NodeClasses"])
}

func karpenterListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		nodePoolGVR:     "NodePoolList",
		nodeClaimGVR:    "NodeClaimList",
		ec2NodeClassGVR: "EC2NodeClassList",
	}
}

func TestListKarpenterInventorySuccess(t *testing.T) {
	np := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodePool",
		"metadata":   map[string]any{"name": "default", "uid": "np-1"},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"nodeClassRef": map[string]any{"name": "default", "kind": "EC2NodeClass", "group": "karpenter.k8s.aws"},
				},
			},
		},
	}}
	nc := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodeClaim",
		"metadata": map[string]any{
			"name":   "claim-1",
			"uid":    "nc-1",
			"labels": map[string]any{karpenterNodePoolLabelKey: "default"},
		},
	}}
	ec2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1",
		"kind":       "EC2NodeClass",
		"metadata":   map[string]any{"name": "default", "uid": "ec2-1"},
		"spec":       map[string]any{"role": "NodeRole", "amiFamily": "AL2023"},
	}}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), karpenterListKinds(), np, nc, ec2)
	pools, claims, classes := listKarpenterInventory(context.Background(), client)
	require.Len(t, pools, 1)
	require.Equal(t, "default", pools[0].Name)
	require.Len(t, claims, 1)
	require.Equal(t, "default", claims[0].NodePool)
	require.Len(t, classes, 1)
	require.Equal(t, "NodeRole", classes[0].Role)
	require.Equal(t, "AL2023", classes[0].AMIFamily)
}

func TestListKarpenterInventorySoftFailForbidden(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), karpenterListKinds())
	client.PrependReactor("list", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(action.GetResource().GroupResource(), "", nil)
	})

	pools, claims, classes := listKarpenterInventory(context.Background(), client)
	require.NotNil(t, pools)
	require.NotNil(t, claims)
	require.NotNil(t, classes)
	require.Empty(t, pools)
	require.Empty(t, claims)
	require.Empty(t, classes)
}

func TestListKarpenterInventorySoftFailNotFound(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), karpenterListKinds())
	client.PrependReactor("list", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "")
	})

	pools, claims, classes := listKarpenterInventory(context.Background(), client)
	require.Empty(t, pools)
	require.Empty(t, claims)
	require.Empty(t, classes)
}

func TestListClusterUnstructuredPagination(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), karpenterListKinds())
	page := 0
	client.PrependReactor("list", "nodepools", func(action clienttesting.Action) (bool, runtime.Object, error) {
		page++
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "karpenter.sh/v1",
					"kind":       "NodePool",
					"metadata": map[string]any{
						"name": fmt.Sprintf("pool-%d", page),
						"uid":  fmt.Sprintf("uid-%d", page),
					},
				}},
			},
		}
		list.SetAPIVersion("karpenter.sh/v1")
		list.SetKind("NodePoolList")
		if page == 1 {
			list.SetContinue("next-page")
		}
		return true, list, nil
	})

	items, err := listClusterUnstructured(context.Background(), client, nodePoolGVR)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "pool-1", items[0].GetName())
	require.Equal(t, "pool-2", items[1].GetName())
	require.Equal(t, 2, page)
}
