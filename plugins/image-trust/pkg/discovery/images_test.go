package discovery

import "testing"

func TestNamespaceIsBlocked(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		blocklist []string
		allowlist []string
		want      bool
	}{
		{
			name:      "blocked namespace wins",
			namespace: "kube-system",
			blocklist: []string{"kube-system"},
			want:      true,
		},
		{
			name:      "allowlist permits namespace",
			namespace: "prod",
			allowlist: []string{"prod"},
			want:      false,
		},
		{
			name:      "allowlist blocks everything else",
			namespace: "dev",
			allowlist: []string{"prod"},
			want:      true,
		},
		{
			name:      "no lists means allowed",
			namespace: "default",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := namespaceIsBlocked(tt.namespace, tt.blocklist, tt.allowlist)
			if got != tt.want {
				t.Fatalf("namespaceIsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
