package util

import "strings"

func ExtractMetadata(obj map[string]interface{}) (string, string, string, string) {
	kind, _ := obj["kind"].(string)
	apiVersion, _ := obj["apiVersion"].(string)
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return apiVersion, kind, "", ""
	}
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	return apiVersion, kind, name, namespace
}

// GetRepoDetails splits the repo name
func GetRepoDetails(repositoryName string) (owner, repoName string) {
	repositorySplit := strings.Split(repositoryName, "/")
	if len(repositorySplit) == 2 {
		return repositorySplit[0], repositorySplit[1]
	}
	return "", repositoryName
}

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
