package handlers

import (
	"log/slog"
	"strings"

	"github.com/ghodss/yaml"
)

func ExtractPoliciesFromMessage(message string) map[string]map[string]string {
	policies := map[string]map[string]string{}
	allPolicies := ""
	if strings.Contains(message, "admission webhook") && strings.Contains(message, "denied the request:") {
		expectedText := "due to the following policies"
		start := strings.Index(message, expectedText)
		if start != -1 {
			start = start + len(expectedText)
			allPolicies = message[start:]
		}
	}
	err := yaml.Unmarshal([]byte(allPolicies), &policies)
	if err != nil {
		slog.Error("Failed to unmarshal policies", "error", err)
		return map[string]map[string]string{}
	}
	return policies
}
