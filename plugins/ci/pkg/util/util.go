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

// ExactlyOneOf looks for atleast one occurance.
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

// RemoveToken sanitizes output to remove token
func RemoveToken(s string) string {
	index := strings.Index(s, "x-access-token")
	index2 := strings.Index(s, "@github.com")
	if index < 0 || index2 < 0 {
		return s
	}
	return strings.ReplaceAll(s, s[index+15:index2], "<TOKEN>")
}
