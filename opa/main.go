package main

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/sirupsen/logrus"

	opa "github.com/fairwindsops/insights-plugins/opa/pkg/opa"
)

const outputFile = "/output/opa.json"

// Output is the format for the output file
type Output struct {
	ActionItems []opa.ActionItem
}

func main() {
	logrus.Info("Starting OPA reporter")
	ctx := context.Background()
	actionItems, runError := opa.Run(ctx)
	if actionItems != nil {
		logrus.Infof("Finished processing OPA checks, found %d Action Items", len(actionItems))

		output := Output{ActionItems: actionItems}
		value, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(outputFile, value, 0644)
		if err != nil {
			panic(err)
		}
	}
	if runError != nil {
		panic(runError)
	}
}
