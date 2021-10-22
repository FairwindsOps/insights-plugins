package main

import (
	"fmt"
	"os"
)

func main() {
	clientset := createClient() // setup client to talk to kubernetes api

	workloads := getWorkloads(clientset) // get list of all relevant workloads
	hpas := getAutoscalers(clientset)    // get all horizontalpodautoscalers
	pdbs := getPdbs(clientset)           // get all poddisruptionbudgets

	problemWorkloads := discoverProblematicWorkloads(workloads, pdbs, hpas) // find ones that violate pdb spec

	problemWorkloads = recommendSolution(problemWorkloads)

	if len(problemWorkloads) == 0 { // do nothing if we have no problems
		fmt.Println("No problems found.")
		os.Exit(0)
	}

	// flatOut(problemWorkloads)
	tableOut(problemWorkloads)
}
