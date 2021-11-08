package report

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Should we import from github.com/fairwindsops/insights/pkg/reports instead
// of defining the same types here?
// RightSizerReportItem shows the right-sizer-item property
type RightSizerReportItem struct {
	Kind              string             `json:"kind"`
	ResourceName      string             `json:"resourceName"`
	ResourceNamespace string             `json:"resourceNamespace"`
	ResourceContainer string             `json:"resourceContainer"`
	StartingMemory    *resource.Quantity `json:"startingMemory"`
	EndingMemory      *resource.Quantity `json:"endingMemory"`
}

// RightSizerReportProperties shows the right-sizer-item property
type RightSizerReportProperties struct {
	Items     []RightSizerReportItem `json:"items"`
	itemsLock *sync.RWMutex
}

// RightSizerReport is a report from right-sizer-report
type RightSizerReport struct {
	Version string
	Report  RightSizerReportProperties
}

func (i RightSizerReportItem) String() string {
	return fmt.Sprintf("%s %s/%s:%s", i.Kind, i.ResourceNamespace, i.ResourceName, i.ResourceContainer)
}

// NewRightSizerReportProperties returns a pointer to a new initialized
// RightSizerReportProperties type.
func NewRightSizerReportProperties() *RightSizerReportProperties {
	p := &RightSizerReportProperties{
		itemsLock: &sync.RWMutex{},
	}
	return p
}

// alreadyHave accepts a RightSizerReportItem and returns true if that item
// already exists in the RightSizerReportProperties.
// ONly kind, namespace, name, and container name are matched.
func (p *RightSizerReportProperties) AlreadyHave(newItem RightSizerReportItem) bool {
	p.itemsLock.RLock()
	defer p.itemsLock.RUnlock()
	for _, item := range p.Items {
		if item.Kind == newItem.Kind && item.ResourceNamespace == newItem.ResourceNamespace && item.ResourceName == newItem.ResourceName && item.ResourceContainer == newItem.ResourceContainer {
			return true
		}
	}
	return false
}

func (p *RightSizerReportProperties) AddItem(newItem RightSizerReportItem) {
	p.itemsLock.Lock()
	defer p.itemsLock.Unlock()
	p.Items = append(p.Items, newItem)
}
