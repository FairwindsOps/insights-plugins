/*
Copyright © 2022 FairwindsOps Inc
*/
package main

import (
	"github.com/fairwindsops/insights-plugins/plugins/opa/cmd"
	opa "github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
)

// Output is the format for the output file
type Output struct {
	ActionItems []opa.ActionItem
}

func main() {
	cmd.Execute()
}
