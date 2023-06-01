package workloads

import (
	"embed"
	"strings"
)

//go:generate cp ../version.txt ./version.txt
//go:embed version.txt
var fs embed.FS

var Version string

func init() {
	c, err := fs.ReadFile("version.txt")
	if err != nil {
		panic(err)
	}
	Version = strings.TrimSpace(string(c))
}
