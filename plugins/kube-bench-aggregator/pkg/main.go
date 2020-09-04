package main

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"

	"github.com/aquasecurity/kube-bench/check"
)

const port = "8080"
const outputFile = "/output/kube-bench.json"

// ID for the Policies category, which is not node specific.
const policiesID = "5"

type kubeBenchModel struct {
	Name     string
	Controls []check.Controls
}

func main() {
	data := map[string]check.Controls{}
	daemonsetName := os.Getenv("DAEMONSET_SERVICE")
	if daemonsetName == "" {
		panic("Must set DAEMONSET_SERVICE")
	}

	// The headless service will return multiple IPs, one for each pod.
	addresses, err := net.LookupHost(daemonsetName)

	if err != nil {
		panic(err)
	}

	for _, address := range addresses {
		response, err := http.Get("http://" + address + ":" + port)

		if err != nil {
			panic(err)
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}

		responseData := kubeBenchModel{}
		err = json.Unmarshal(body, &responseData)
		if err != nil {
			panic(err)
		}

		for _, control := range responseData.Controls {
			key := control.ID
			// ID 5 "Policies" should be the same for every node.
			if key != policiesID {
				key = responseData.Name + "/" + key
			}
			data[key] = control
		}

		response.Body.Close()
	}
	outputBytes, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(outputFile, []byte(outputBytes), 0644)
	if err != nil {
		panic(err)
	}

}
