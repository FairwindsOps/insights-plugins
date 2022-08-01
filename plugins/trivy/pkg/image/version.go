package image

import (
	"sort"

	semver "github.com/Masterminds/semver/v3"
)

type Versions []*semver.Version

func (s Versions) Len() int {
	return len(s)
}

func (s Versions) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Versions) Less(i, j int) bool {
	return s[i].LessThan(s[j])
}

func (s Versions) ToStringSlice() []string {
	tags := []string{}
	for _, v := range s {
		tags = append(tags, v.Original())
	}
	return tags
}

// Sort sorts the given slice of Version
func Sort(versions []*semver.Version) {
	sort.Sort(Versions(versions))
}
