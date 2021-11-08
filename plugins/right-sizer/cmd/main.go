package main

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/controller"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var opts struct {
	Verbose int  `env:"VERBOSE" short:"v" long:"verbose" description:"Show verbose debug information"`
	Version bool `long:"version" description:"Print version information"`
}

// VERSION represents the current version of the release.
const VERSION = "v1.2.0"

func main() {
	util.ParseArgs(&opts)

	if opts.Version {
		printVersion()
		return
	}

	stopChan := make(chan struct{})
	/*ifetch: This was in the original oom-kill-event-generator code, but ends
	* up causing the same channel be closed twice.
	This created a panic when a signal (SIGTerm) was received.
	This is now commented out, in favor of the channel being closed below via the struct member.*/
	// util.InstallSignalHandler(stopChan)

	eventGenerator := controller.NewController(stopChan)
	util.InstallSignalHandler(eventGenerator.Stop)

	http.Handle("/metrics", promhttp.Handler())
	// addr := fmt.Sprintf("0.0.0.0:10254")
	addr := fmt.Sprintf("localhost:10254")
	go func() { glog.Fatal(http.ListenAndServe(addr, nil)) }()

	err := eventGenerator.Run()
	if err != nil {
		glog.Fatal(err)
	}
}

func printVersion() {
	fmt.Printf("kubernetes-oom-event-generator %s %s/%s %s\n", VERSION, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
