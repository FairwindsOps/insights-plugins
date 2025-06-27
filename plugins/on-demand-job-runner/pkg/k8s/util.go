package k8s

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

func mergeEnvs(existing, overrides []corev1.EnvVar) []corev1.EnvVar {
	envMap := map[string]corev1.EnvVar{}
	for _, env := range existing {
		envMap[env.Name] = env
	}
	for _, env := range overrides {
		envMap[env.Name] = env
	}

	finalEnv := make([]corev1.EnvVar, 0, len(envMap))
	for _, v := range envMap {
		finalEnv = append(finalEnv, v)
	}
	return finalEnv
}

func GenerateJobName(cronJobName string, id int64) string {
	random := uuid.New().String()[:8]

	name := fmt.Sprintf("%s-%d-%s", cronJobName, id, random)
	name = strings.ToLower(name)

	if len(name) > 63 {
		name = name[:63]
	}

	return name
}
