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
	Verbose                 int    `env:"VERBOSE" short:"v" long:"verbose" description:"Show verbose debug information"`
	StateConfigMapNameSpace string `env:"RIGHTSIZER_STATE_CONFIGMAP_NAMESPACE" short:"S" long:"state-configmap-namespace" description:"The Kubernetes namespace for the ConfigMap resource used to persist report state inbetween controller restarts."`
	StateConfigMapName      string `env:"RIGHTSIZER_STATE_CONFIGMAP_NAME" short:"s" long:"state-configmap-name" description:"The name of the ConfigMap resource used to persist report state inbetween controller restarts."`
	UpdateMemoryLimits      bool   `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS" short:"m" long:"update-memory-limits" description:"Update memory limits of pod-controllers whos pods get OOM-killed. Updating stops once 2X the original limits have been reached."`
}

func main() {
	util.ParseArgs(&opts)

	stopChan := make(chan struct{})
	eventGenerator := controller.NewController(stopChan,
		controller.WithStateConfigMapNameSpace(opts.StateConfigMapNameSpace),
		controller.WithStateConfigMapName(opts.StateConfigMapName),
		controller.WithUpdateMemoryLimits(opts.UpdateMemoryLimits),
	)
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
