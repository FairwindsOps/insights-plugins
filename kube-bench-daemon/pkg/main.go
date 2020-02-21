package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aquasecurity/kube-bench/check"
)

const port = "8080"
const outputFile = "/output/kubesec.json"

type kubeBenchModel struct {
	Name     string
	Controls []check.Controls
}

var model = kubeBenchModel{}

func getReportsHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(model)
}

func updateModel() {
	log.Println("Updating data.")
	cmd := exec.Command("kube-bench", "--json")
	response, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalln(err, string(response))
	}
	decoder := json.NewDecoder(strings.NewReader(string(response)))
	allControls := make([]check.Controls, 0)
	for {
		var controls check.Controls
		err = decoder.Decode(&controls)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		allControls = append(allControls, controls)
	}
	model.Controls = allControls
	log.Println("Data updated.")

}

func main() {
	log.Println("Starting:")
	model.Name = os.Getenv("NODE_NAME")
	model.Controls = make([]check.Controls, 0)

	http.HandleFunc("/", getReportsHandler)

	ticker := time.NewTicker(2 * time.Hour)
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		updateModel()
		for {
			select {
			case <-ticker.C:
				updateModel()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	log.Fatal(http.ListenAndServe(":8080", nil))

}
