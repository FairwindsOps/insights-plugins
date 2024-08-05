// version is imported by other packages of this plugin, to determine
// the plugin version, which is obtained from the version.txt file.
package version

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

var (
	//go:embed version.txt
	version string
)

func init() {
	version = strings.TrimSpace(version)
	// Make sure the version read from version.txt is valid.
	versionRegexp := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !versionRegexp.MatchString(version) {
		panic(fmt.Sprintf("Version %q is an invalid version number and cannot be submitted with report data", version))
	}
}

func String() string {
	return version
}
