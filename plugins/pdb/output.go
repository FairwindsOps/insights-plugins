package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

func jsonOut(problemWorkloads []ProblemWorkload) error {
	output, err := json.Marshal(problemWorkloads)
	if err != nil {
		return errors.New("couldn't marshal list of problematic PDBs")
	}

	fmt.Println(string(output))

	return nil
}
