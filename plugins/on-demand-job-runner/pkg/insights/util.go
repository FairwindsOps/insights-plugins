package insights

import (
	"regexp"
	"strings"
)

func ToEnvVarFormat(input string) string {
	// Step 1: Insert underscore between lowercase/digit and uppercase letters (e.g., jobID -> job_ID)
	re1 := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	s1 := re1.ReplaceAllString(input, `${1}_${2}`)

	// Step 2: Insert underscore between acronym and normal word (e.g., HTTPRequest -> HTTP_Request)
	re2 := regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	s2 := re2.ReplaceAllString(s1, `${1}_${2}`)

	return strings.ToUpper(s2)
}
