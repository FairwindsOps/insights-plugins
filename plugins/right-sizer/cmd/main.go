package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/controller"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/report"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var opts struct {
	Verbose                      int     `env:"VERBOSE" short:"v" long:"verbose" description:"Show verbose debug information"`
	StateConfigMapNamespace      string  `env:"RIGHTSIZER_STATE_CONFIGMAP_NAMESPACE" short:"S" long:"state-configmap-namespace" description:"The Kubernetes namespace for the ConfigMap resource used to persist report state inbetween controller restarts."`
	StateConfigMapName           string  `env:"RIGHTSIZER_STATE_CONFIGMAP_NAME" short:"s" long:"state-configmap-name" description:"The name of the ConfigMap resource used to persist report state inbetween controller restarts."`
	UpdateMemoryLimits           bool    `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS" short:"m" long:"update-memory-limits" description:"Update memory limits of pod-controllers of OOM-killed pods. Updating stops once the threshold is reached defined by the --max-memory-update-limits-factor option."`
	UpdateMemoryLimitsMultiplier float64 `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS_MULTIPLIER" short:"u" long:"update-memory-limits-multiplier" description:"The multiplier used to increase memory limits, in response to an OOM-kill. This value is multiplied by the limits of the OOM-killed pod. This option is only used if --update-memory-limits is enabled."`
	MaxMemoryLimitsUpdateFactor  float64 `env:"RIGHTSIZER_MAX_MEMORY_LIMITS_UPDATE_FACTOR" short:"U" long:"max-memory-update-limits-factor" description:"The multiplier used to calculate the maximum value to update memory limits. This value is multiplied by the starting memory limits of the first OOM-killed pod seen by this controller. This option is only used if --update-memory-limits is enabled."`
}

func main() {
	util.ParseArgs(&opts)

	if opts.MaxMemoryLimitsUpdateFactor <= opts.UpdateMemoryLimitsMultiplier {
		fmt.Fprintf(os.Stderr, "Please specify a MaxMemoryLimitsUpdateFactor (%.2f) larger than UpdateMemoryLimitsMultiplier (%.2f).\n", opts.MaxMemoryLimitsUpdateFactor, opts.UpdateMemoryLimitsMultiplier)
		os.Exit(1)
	}

	stopChan := make(chan struct{})
	kubeClientResources := util.Clientset() // Create client, dynamic client, RESTMapper...
	reportBuilder := report.NewRightSizerReportBuilder(kubeClientResources,
		report.WithStateConfigMapNamespace(opts.StateConfigMapNamespace),
		report.WithStateConfigMapName(opts.StateConfigMapName),
	)
	eventController := controller.NewController(stopChan,
		kubeClientResources,
		reportBuilder,
		controller.WithUpdateMemoryLimits(opts.UpdateMemoryLimits),
		controller.WithUpdateMemoryLimitsMultiplier(opts.UpdateMemoryLimitsMultiplier),
		controller.WithMaxMemoryLimitsUpdateFactor(opts.MaxMemoryLimitsUpdateFactor),
	)
	util.InstallSignalHandler(eventController.Stop)

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

	err := eventController.Run()
	if err != nil {
		glog.Fatal(err)
	}
}
