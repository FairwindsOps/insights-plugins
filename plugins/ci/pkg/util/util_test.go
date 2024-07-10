package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveTokensAndPassword(t *testing.T) {
	assert.Equal(t, "https://x-access-token:<TOKEN>@github.com/FairwindsOps/charts", RemoveTokensAndPassword("https://x-access-token:my-secret-token@github.com/FairwindsOps/charts"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-creds <CREDENTIALS> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-creds username:password docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-registry-token <TOKEN> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-registry-token my-secret-token docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
}

func TestExtractMetadata(t *testing.T) {
	type want struct {
		apiVersion, kind, name, namespace string
		labels                            map[string]string
	}

	type test struct {
		name  string
		input map[string]any
		want  want
	}

	tests := []test{
		{
			name: "all-fields",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "my-pod",
					"namespace": "default",
					"labels": map[string]any{
						"app": "my-app",
					},
				},
			},
			want: want{
				apiVersion: "v1",
				kind:       "Pod",
				name:       "my-pod",
				namespace:  "default",
				labels: map[string]string{
					"app": "my-app",
				},
			},
		},
		{
			name: "no-labels",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      "my-pod",
					"namespace": "default",
				},
			},
			want: want{
				apiVersion: "v1",
				kind:       "Pod",
				name:       "my-pod",
				namespace:  "default",
				labels:     nil,
			},
		},
		{
			name: "no-metadata",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			want: want{
				apiVersion: "v1",
				kind:       "Pod",
				name:       "",
				namespace:  "",
				labels:     nil,
			},
		},
		{
			name: "no-namespace",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name": "my-pod",
				},
			},
			want: want{
				apiVersion: "v1",
				kind:       "Pod",
				name:       "my-pod",
				namespace:  "",
				labels:     nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			apiVersion, kind, name, namespace, labels := ExtractMetadata(test.input)
			assert.Equal(tt, test.want.apiVersion, apiVersion)
			assert.Equal(tt, test.want.kind, kind)
			assert.Equal(tt, test.want.name, name)
			assert.Equal(tt, test.want.namespace, namespace)
			assert.Equal(tt, test.want.labels, labels)
		})
	}

}
