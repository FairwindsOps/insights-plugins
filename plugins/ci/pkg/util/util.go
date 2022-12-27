package util

import (
	"encoding/json"
	"strings"
)

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

// RemoveTokensAndPassword sanitizes output to remove token
func RemoveTokensAndPassword(s string) string {
	// based on x-access-token
	index := strings.Index(s, "x-access-token")
	index2 := strings.Index(s, "@github.com")
	if index > 0 && index2 > 0 {
		return strings.ReplaceAll(s, s[index+15:index2], "<TOKEN>")
	}

	// based on --src-creds
	index = strings.Index(s, "--src-creds")
	if index > 0 {
		f := index + 12 // start of credentials
		l := strings.Index(s, " docker")
		return strings.ReplaceAll(s, s[f:l], "<CREDENTIALS>")
	}

	// based on --src-registry-token
	index = strings.Index(s, "--src-registry-token")
	if index > 0 {
		f := index + 21 // start of credentials
		l := strings.Index(s, " docker")
		return strings.ReplaceAll(s, s[f:l], "<TOKEN>")
	}

	return s
}

func PrettyPrint(i interface{}) string {
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
