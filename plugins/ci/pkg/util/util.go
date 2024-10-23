package util

import (
	"encoding/json"
	"strings"
)

func ExtractMetadata(obj map[string]any) (apiVersion, kind, name, namespace string, labels map[string]string) {
	kind, _ = obj["kind"].(string)
	apiVersion, _ = obj["apiVersion"].(string)
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return apiVersion, kind, "", "", nil
	}
	name, _ = metadata["name"].(string)
	namespace, _ = metadata["namespace"].(string)
	labelsMap, ok := metadata["labels"].(map[string]any)
	if ok && len(labelsMap) > 0 {
		labels = make(map[string]string, len(labelsMap))
		for k, v := range labelsMap {
			if s, ok := v.(string); ok {
				labels[k] = s
			}
		}
	}
	return apiVersion, kind, name, namespace, labels
}

// GetRepoDetails splits the repo name
func GetRepoDetails(repositoryName string) (owner, repoName string) {
	repositorySplit := strings.Split(repositoryName, "/")
	if len(repositorySplit) == 2 {
		return repositorySplit[0], repositorySplit[1]
	}
	return "", repositoryName
}

// ExactlyOneOf looks for at least one occurrence.
func ExactlyOneOf(inputs ...bool) bool {
	foundAtLeastOne := false
	for _, input := range inputs {
		if input {
			if foundAtLeastOne {
				return false
			}
			foundAtLeastOne = true
		}
	}
	return foundAtLeastOne
}

func PrettyPrint(i any) string {
	s, _ := json.Marshal(i)
	return string(s)
}

func ReverseMap(m map[string]string) map[string]string {
	n := make(map[string]string, len(m))
	for k, v := range m {
		n[v] = k
	}
	return n
}
