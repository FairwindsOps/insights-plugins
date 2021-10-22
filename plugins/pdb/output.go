package main

import (
	"fmt"
	"strconv"

	prettytable "github.com/tatsushid/go-prettytable"
)

func flatOut(problemWorkloads []ProblemWorkload) {
	for _, item := range problemWorkloads { // print list of problems
		hpaMinReplicas := strconv.Itoa(item.Hpa.MinReplicas)
		hpaMaxReplicas := strconv.Itoa(item.Hpa.MaxReplicas)
		fmt.Print(
			"\n",
			"Namespace: ", item.Workload.Namespace, "\n",
			"Kind/Name: ", item.Workload.Kind+"/"+item.Workload.Name, "\n",
			"Replicas: ", item.Workload.Replicas, "\n",
			"PDB Kind/Name: ", item.Pdb.Kind+"/"+item.Pdb.Name, "\n",
			"PDB min: ", item.Pdb.MinAvailable, "\n",
			"PDB max: ", item.Pdb.MaxUnavailable, "\n",
			"HPA (min, max): ", item.Hpa.Kind+"/"+item.Hpa.Name+" ("+hpaMinReplicas+","+hpaMaxReplicas+")", "\n",
			"Recommendation: ", item.Recommendation, "\n",
			"\n",
		)
	}
}

func tableOut(problemWorkloads []ProblemWorkload) {
	table, err := prettytable.NewTable(
		prettytable.Column{Header: "Namespace"},
		prettytable.Column{Header: "Kind/Name"},
		prettytable.Column{Header: "Replicas"},
		prettytable.Column{Header: "PDB Kind/Name"},
		prettytable.Column{Header: "PDB Min"},
		prettytable.Column{Header: "PDB Max"},
		prettytable.Column{Header: "HPA (min,max)"},
		prettytable.Column{Header: "Recommendation"},
	)
	if err != nil {
		panic(err)
	}
	table.Separator = " | "
	for _, pwl := range problemWorkloads {
		hpaMinReplicas := strconv.Itoa(pwl.Hpa.MinReplicas)
		hpaMaxReplicas := strconv.Itoa(pwl.Hpa.MaxReplicas)
		table.AddRow(
			pwl.Workload.Namespace,
			pwl.Workload.Kind+"/"+pwl.Workload.Name,
			pwl.Workload.Replicas,
			pwl.Pdb.Kind+"/"+pwl.Pdb.Name,
			pwl.Pdb.MinAvailable,
			pwl.Pdb.MaxUnavailable,
			pwl.Hpa.Kind+"/"+pwl.Hpa.Name+" ("+hpaMinReplicas+","+hpaMaxReplicas+")",
			pwl.Recommendation,
		)
	}
	table.Print()
}
