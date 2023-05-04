package models

import (
	"fmt"
	"strings"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// persistentVolumeClaim holds minimal information about a PVC to track
// capacity and how many pods share a claim.
type persistentVolumeClaim struct {
	name     string
	capacity int64    // Represented in bytes.
	refs     []string // Pods that reference this PVC, of the form namespace/name.
}

// numRefs returns the number of references that pods have to this persistentVolumeClaim.
func (PVC persistentVolumeClaim) numRefs() int {
	return len(PVC.refs)
}

// StorageInfo holds persistentVolumeClains and the total capacity of PVCs for
// each Pod.
type StorageInfo struct {
	pvcs                []*persistentVolumeClaim
	totalCapacityPerPod map[string]int64 // keyed by the pod namespace/name.
}

// NewStorageInfoFromCluster fetches PersistentVolumeClaim resources from
// in-cluster, and includes them in the newly-constructed returned StorageInfo type.
func NewStorageInfoFromCluster(client *controller.Client) (s *StorageInfo, err error) {
	unstructuredPVCs, err := client.GetAllPersistentVolumeClaims("")
	if err != nil {
		return nil, err
	}
	return NewStorageInfoFromUnstructuredPVCs(unstructuredPVCs), nil
}

// NewStorageInfoFromUnstructuredVPCs accepts PersistentVolumeClaim resources
// as a slice of the unstructured.Unstructured type,
// and returns them in a new StorageInfo type.
func NewStorageInfoFromUnstructuredPVCs(unstructuredPVCs []unstructured.Unstructured) *StorageInfo {
	s := &StorageInfo{}
	for _, unstructuredPVC := range unstructuredPVCs {
		PVCName, foundName, err := unstructured.NestedString(unstructuredPVC.UnstructuredContent(), "metadata", "name")
		if err != nil {
			logrus.Warnf("skipping PersistentVolumeClaim, unable to get metadata.name from unstructured resource: error=%v, PVC=%#v", err, unstructuredPVC.UnstructuredContent())
			continue
		}
		if !foundName || PVCName == "" {
			logrus.Warnf("skipping PersistentVolumeClaim, unable to get metadata.name from unstructured resource: %#v", unstructuredPVC.UnstructuredContent())
			continue
		}
		PVCCapacityStr, foundCapacity, err := unstructured.NestedString(unstructuredPVC.UnstructuredContent(), "status", "capacity", "storage")
		if err != nil {
			logrus.Warnf("skipping PersistentVolumeClaim, unable to get status.capacity.storage from unstructured resource: error=%v, PVC=%#v", err, unstructuredPVC.UnstructuredContent())
			continue
		}
		if !foundCapacity {
			logrus.Warnf("skipping PersistentVolumeClaim, unable to get storage.capacity.storage from unstructured resource: %#v", unstructuredPVC.UnstructuredContent())
			continue
		}
		PVCCapacityInt, err := capacityString2Int(PVCCapacityStr)
		if err != nil {
			logrus.Warnf("skipping PersistentVolumeClaim, unable to convert capacity %q into bytes: %v", PVCCapacityStr, err)
			continue
		}
		PVC := &persistentVolumeClaim{
			name:     PVCName,
			capacity: PVCCapacityInt,
		}
		s.pvcs = append(s.pvcs, PVC)
	}
	return s
}

// PVCsAsString facilitates printing a slice of persistentVolumeClaim
// types for debugging.
func (s StorageInfo) PVCsAsString() string {
	output := fmt.Sprintf("%d PVCS:\n", len(s.pvcs))
	for i, v := range s.pvcs {
		output += fmt.Sprintf("PVC %d: %#v\n", i, *v)
	}
	return output
}

// NumPVCs returns the number of PersistentVolumeClains fetched from
// in-cluster by NewStorageInfo().
func (s StorageInfo) NumPVCs() int {
	return len(s.pvcs)
}

// PVCByName returns a persistentVolumeClaim matching the specified name. IF
// no PVC is found, nil is returned.
func (s StorageInfo) PVCByName(name string) *persistentVolumeClaim {
	for _, PVC := range s.pvcs {
		if PVC.name == name {
			return PVC
		}
	}
	return nil
}

// AddPVCRef associates the specified Pod name, with the specified
// persistentVolumeClaim, and adds the PVC capacity to the total capacity for
// the Pod.
// The persistenvVolumeClaim name is expected to exist in-cluster, as
// discovered by NewStorageInfo().
// The Pod name is of the form {namespace name}/{pod name}.
func (s *StorageInfo) AddPVCRef(PVCName, podKey string) {
	if s.NumPVCs() == 0 {
		logrus.Warnf("cannot add reference %q to PersistentVolumeClaim %q  when there are 0 PVCs, was NewStorageInfo() called first?", podKey, PVCName)
		return
	}
	if PVCName == "" || podKey == "" {
		logrus.Warnf("cannot add reference %q to PersistentVolumeClaim %q  when either the pod-key (reference) or PVC name are empty", podKey, PVCName)
		return
	}
	PVC := s.PVCByName(PVCName)
	if PVC == nil {
		logrus.Warnf("cannot add reference %q to PersistentVolumeClaim %q because the PVC was not found in-cluster", podKey, PVCName)
		return
	}
	PVC.refs = append(PVC.refs, podKey)
	if s.totalCapacityPerPod == nil {
		s.totalCapacityPerPod = make(map[string]int64)
	}
	s.totalCapacityPerPod[podKey] += PVC.capacity
	logrus.Debugf("updated total storage capacity for pod %q to %d, and added that pod as a reference for PersistentVolumeClaim %s", podKey, s.totalCapacityPerPod[podKey], PVC.name)
}

// manufactureMetrics spreads the capacity of PersistentVolumeClaims across Pods that share that
// Claim. The same value is used to fill metric values for the range of time
// specified in the prometheusV1.Range.
func (s *StorageInfo) ManufactureMetrics(r prometheusV1.Range) model.Matrix {
	newMetrics := make(model.Matrix, 0)
	for _, pvc := range s.pvcs {
		for _, ref := range pvc.refs { // Iterate pods (namespace/name) that reference this PVC
			refFields := strings.Split(ref, "/") // split namespace and pod-name to include in metrics
			if len(refFields) < 2 {
				logrus.Warnf("cannot split PersistentVolumeClaim ref %q by slash, to get namespace and name, this PersistentVolumeClaim reference will not have metrics", ref)
				continue
			}
			newSample := &model.SampleStream{}
			newSample.Metric = make(model.Metric)
			newSample.Metric["namespace"] = model.LabelValue(refFields[0])
			newSample.Metric["pod"] = model.LabelValue(refFields[1])
			refMetricValue := int64(float64(s.totalCapacityPerPod[ref]))
			newSample.Values = []model.SamplePair{
				{
					Timestamp: model.Time(r.Start.UnixMilli()),
					Value:     model.SampleValue(refMetricValue),
				},
			}
			newMetrics = append(newMetrics, newSample)
			logrus.Debugf("using value %d for storage-capacity metric pod=%s, PVC=%s", refMetricValue, ref, pvc.name)
		}
	}
	return newMetrics
}

// capacityString2Int uses a resource.Quantity type to convert a capacity with
// units (5 Gi) into bytes.
func capacityString2Int(capacityStr string) (capacityInt int64, err error) {
	capacityQuantity, err := resource.ParseQuantity(capacityStr)
	if err != nil {
		return 0, fmt.Errorf("unable to convert the capacity string %q to a resource.Quantity: %v", capacityStr, err)
	}
	capacityInt, ok := capacityQuantity.AsInt64()
	if !ok {
		return 0, fmt.Errorf("unable to get the integer form of a capacity %q via a resource.Quantity type %#v", capacityStr, capacityQuantity)
	}
	logrus.Debugf("parsed capacity %q as %d bytes", capacityStr, capacityInt)
	return capacityInt, nil
}
