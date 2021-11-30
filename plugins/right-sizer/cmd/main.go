package main

import (
	"net/http"
	"os"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/controller"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var opts struct {
	Verbose int `env:"VERBOSE" short:"v" long:"verbose" description:"Show verbose debug information"`
}

func main() {
	util.ParseArgs(&opts)

	stopChan := make(chan struct{})
	eventGenerator := controller.NewController(stopChan)
	util.InstallSignalHandler(eventGenerator.Stop)

	http.Handle("/metrics", promhttp.Handler())
	var addr string
	inKube := os.Getenv("KUBERNETES_SERVICE_HOST")
	if inKube != "" {
		addr = "0.0.0.0:10254"
	} else {
		// Use localhost outside of Kubernetes, to avoid Mac OS
		// accept-incoming-network-connection warnings.
		addr = "localhost:10254"
	}
	go func() { glog.Fatal(http.ListenAndServe(addr, nil)) }()

	err := eventGenerator.Run()
	if err != nil {
		glog.Fatal(err)
	}
}
