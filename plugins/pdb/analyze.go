package main

import (
	"fmt"
	"strconv"
	"strings"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	policyv1 "k8s.io/api/policy/v1"
)

func discoverProblematicWorkloads(workloads []Workload, pdbs *policyv1.PodDisruptionBudgetList, hpas *autoscalingv1.HorizontalPodAutoscalerList) []ProblemWorkload {
	problemWorkloads := []ProblemWorkload{}

	for _, pdb := range pdbs.Items {
		if pdb.Status.DisruptionsAllowed > 1 || pdb.Status.CurrentHealthy < 1 { // skip pdb that won't cause issues
			continue
		}

		if (pdb.Status.DisruptionsAllowed < 1 && pdb.Status.CurrentHealthy > 0) || pdb.Status.DesiredHealthy >= pdb.Status.CurrentHealthy { // this is a problem
			// fmt.Println("pdb:", pdb.Namespace, pdb.Name, pdb.Status.DisruptionsAllowed)
			for pdbLabelK, pdbLabelV := range pdb.Spec.Selector.MatchLabels {
				for _, workload := range workloads {
					if workload.Namespace != pdb.ObjectMeta.Namespace {
						continue
					}

					if len(workload.Labels) == 0 {
						continue
					}
					if workload.Kind == "ReplicaSet" && len(workload.OwnerReferences) > 0 { // replicaset is owned by something else (probably a deployment)
						continue
					}

					for workloadLabelK, workloadLabelV := range workload.Labels {
						if pdbLabelK == workloadLabelK && pdbLabelV == workloadLabelV {
							_hpa := Hpa{}
							for _, hpa := range hpas.Items {
								if workload.ApiVersion == hpa.Spec.ScaleTargetRef.APIVersion &&
									hpa.Spec.ScaleTargetRef.Kind == workload.Kind &&
									hpa.Spec.ScaleTargetRef.Name == workload.Name &&
									hpa.ObjectMeta.Namespace == workload.Namespace &&
									pdb.Status.DesiredHealthy >= workload.Replicas {
									// _hpa = "yes"
									_hpa = Hpa{
										Kind:        hpa.Kind,
										Namespace:   hpa.ObjectMeta.Namespace,
										Name:        hpa.ObjectMeta.Name,
										MinReplicas: int(*hpa.Spec.MinReplicas),
										MaxReplicas: int(hpa.Spec.MaxReplicas),
									}
								}
							}

							_min := pdb.Spec.MinAvailable.String()                       // shorthand
							_minNoPercent := strings.ReplaceAll(_min, "%", "")           // without % sign
							minavail_float, err := strconv.ParseFloat(_minNoPercent, 64) // change to float64
							if err != nil {                                              // check for errors converting
								fmt.Println(err)
							}
							minavail_float = minavail_float * 0.01 // make decimal from percentage
							// if strings.Contains(_min, "%") {
							// 	fmt.Println("%")
							// 	// minavail_string =  // take out %
							// 	// fmt.Println(minavail_float)
							// } else {
							// 	minavail_float = float64(pdb.Spec.MinAvailable.IntValue())
							// }
							if !strings.Contains(_min, "%") {
								minavail_float = float64(pdb.Spec.MinAvailable.IntValue())
							}
							problemWorkloads = append(problemWorkloads, ProblemWorkload{
								Pdb: Pdb{
									Kind:         pdb.Kind,
									Namespace:    pdb.Namespace,
									Name:         pdb.Name,
									MinAvailable: float64(minavail_float),
									// MaxUnavailable: float32(pdb.Spec.MaxUnavailable.IntValue()),
									DesiredHealthy: pdb.Status.DesiredHealthy,
								},
								Workload: workload,
								Hpa:      _hpa,
							})
						}
					}
					// fmt.Println(workload.Name, pdbLabelK, pdbLabelV)
				}
			}
		}
	}

	return problemWorkloads
}

func recommendSolution(problemWorkloads []ProblemWorkload) []ProblemWorkload {
	for i, pwl := range problemWorkloads {
		recommendation := "Further human analysis" // unknown what should be done to fix the problem

		// PDB w/ percentage
		if int(pwl.Pdb.MinAvailable) == 0 || int(pwl.Pdb.MaxUnavailable) == 0 { // FIXME: check if int(nil) == 0 or TypeError
			recommendation = "Use an integer instead of percentage, or increase number of replicas"
			if len(pwl.Hpa.Name) > 0 { // there is an HPA in play
				recommendation = "Use an integer instead of a percentage, or increase HPA minimum replicas"
			}
		} else

		// hpa set too low
		if pwl.Pdb.DesiredHealthy == int32(pwl.Hpa.MinReplicas) { // even if all pods are healthy, HPA might scale too far down
			recommendation = "Increase the HPA minimum replicas"
		} else

		// workload replicas too low
		if len(pwl.Hpa.Name) == 0 && pwl.Pdb.DesiredHealthy >= pwl.Workload.Replicas {
			recommendation = "Increase replicas"
		}

		// set final recommendation
		problemWorkloads[i].Recommendation = recommendation
	}
	return problemWorkloads
}
