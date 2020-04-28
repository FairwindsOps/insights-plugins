package main

import (
	"context"
	"time"

	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACReport report
type RBACReport struct {
	ServerVersion       string                  `json:"service_version,omitempty"`
	CreationTime        time.Time               `json:"creation_time,omitempty"`
	Roles               []v1.Role               `json:"roles,omitempty"`
	RoleBindings        []v1.RoleBinding        `json:"role_bindings,omitempty"`
	ClusterRoles        []v1.ClusterRole        `json:"cluster_roles,omitempty"`
	ClusterRoleBindings []v1.ClusterRoleBinding `json:"cluster_role_bindings,omitempty"`
}

// CreateResourceProviderFromAPI creates a new ResourceProvider from an existing k8s interface
func CreateResourceProviderFromAPI(ctx context.Context, kube kubernetes.Interface, clusterName string) (*RBACReport, error) {
	listOpts := metav1.ListOptions{}
	serverVersion, err := kube.Discovery().ServerVersion()

	roles := []v1.Role{}
	roleBindings := []v1.RoleBinding{}
	clusterRoles := []v1.ClusterRole{}
	clusterRoleBindings := []v1.ClusterRoleBinding{}

	kubeClusterRoleBindings, err := kube.RbacV1().ClusterRoleBindings().List(ctx, listOpts)
	clusterRoleBindings = kubeClusterRoleBindings.Items

	kubeClusterRoles, err := kube.RbacV1().ClusterRoles().List(ctx, listOpts)
	clusterRoles = kubeClusterRoles.Items

	kubeRoleBindings, err := kube.RbacV1().RoleBindings("").List(ctx, listOpts)
	roleBindings = kubeRoleBindings.Items

	kubeRoles, err := kube.RbacV1().Roles("").List(ctx, listOpts)
	roles = kubeRoles.Items

	rbacReport := RBACReport{
		ServerVersion:       serverVersion.Major + "." + serverVersion.Minor,
		CreationTime:        time.Now(),
		Roles:               roles,
		RoleBindings:        roleBindings,
		ClusterRoles:        clusterRoles,
		ClusterRoleBindings: clusterRoleBindings,
	}

	return &rbacReport, err
}
