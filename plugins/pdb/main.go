package main

func main() {
	clientset := createClient() // setup client to talk to kubernetes api

	workloads := getWorkloads(clientset) // get list of all relevant workloads
	hpas := getAutoscalers(clientset)    // get all horizontalpodautoscalers
	pdbs := getPdbs(clientset)           // get all poddisruptionbudgets

	problemWorkloads := discoverProblematicWorkloads(workloads, pdbs, hpas) // find ones that violate pdb spec

	problemWorkloads = recommendSolution(problemWorkloads) // inject remediation recommendations into each object

	jsonOut(problemWorkloads) // print any findings (empty list if none found)
}
