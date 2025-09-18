package watcher

import (
	_ "embed"
	"strings"
)

var (
	//go:embed version.txt
	content []byte
	Version string
)

func init() {
	Version = strings.TrimSpace(string(content))
}
