package registry

import (
	"strings"
)

// RemapMirror rewrites a registry host using configured mirror→upstream mappings.
// Signatures are typically stored on the upstream registry; mirror hosts are pull-through caches.
func RemapMirror(ref string, mirrors map[string]string) string {
	if ref == "" || len(mirrors) == 0 {
		return ref
	}
	for mirror, upstream := range mirrors {
		if mirror == "" || upstream == "" {
			continue
		}
		if strings.HasPrefix(ref, mirror+"/") {
			return upstream + strings.TrimPrefix(ref, mirror)
		}
		if ref == mirror {
			return upstream
		}
	}
	return ref
}
