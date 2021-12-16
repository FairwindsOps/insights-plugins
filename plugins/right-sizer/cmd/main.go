package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/controller"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/report"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var opts struct {
	Verbose                       int           `env:"VERBOSE" short:"v" long:"verbose" description:"Show verbose debug information"`
	StateConfigMapNamespace       string        `env:"RIGHTSIZER_STATE_CONFIGMAP_NAMESPACE" short:"S" long:"state-configmap-namespace" description:"The Kubernetes namespace for the ConfigMap resource used to persist report state inbetween controller restarts."`
	StateConfigMapName            string        `env:"RIGHTSIZER_STATE_CONFIGMAP_NAME" short:"s" long:"state-configmap-name" description:"The name of the ConfigMap resource used to persist report state inbetween controller restarts."`
	Namespaces                    []string      `env:"RIGHTSIZER_NAMESPACES" short:"N" long:"namespaces" env-delim:"," description:"Only respond to OOM-killed containers in these Kubernetes namespaces. By default, all namespaces are allowed. This applies both to alerting on OOM-kills and modifying memory limits if --update-memory-limits is enabled. Use this option multiple times to specify multiple namespaces. IF setting namespaces via the environment variable, separate namespaces by a comma."`
	ResetOOMsWindow               time.Duration `env:"RIGHTSIZER_RESET_OOMS_WINDOW" short:"w" long:"reset-ooms-window" description:"The amount of time after which an item will be removed from the report, if no OOM-kills have been seen. Specify this as a time duration, such as 24H."`
	UpdateMemoryLimits            bool          `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS" short:"m" long:"update-memory-limits" description:"Update the memory limits of resources, such as deployments, that own pods that have been OOM-killed. Updating stops once the threshold is reached defined by the --update-memory-limits-max option. To specify which namespaces are allowed to be updated, see the --namespaces option."`
	UpdateMemoryLimitsMinimumOOMs int64         `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS_MIN_OOMS" short:"n" long:"update-memory-limits-min-ooms" description:"The number of OOM-kills required before the memory limits will be updated. This option is only used if --update-memory-limits is enabled."`
	UpdateMemoryLimitsIncrement   float64       `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS_INCREMENT" short:"u" long:"update-memory-limits-increment" description:"The multiplier used to increase memory limits, in response to an OOM-kill. This value is multiplied by the limits of the OOM-killed container. This option is only used if --update-memory-limits is enabled. IF this option is specified, the update-memory-limits-max must also be specified."`
	UpdateMemoryLimitsMax         float64       `env:"RIGHTSIZER_UPDATE_MEMORY_LIMITS_MAX" short:"U" long:"update-memory-limits-max" description:"The multiplier used to calculate the maximum value to update memory limits. This value is multiplied by the starting memory limits of the first OOM-killed container seen by this controller. This option is only used if --update-memory-limits is enabled. If this option is specified, the --update-memory-limits-increment must also be specified."`
}

func main() {
	util.ParseArgs(&opts)

	if opts.UpdateMemoryLimitsMax > 0.0 && opts.UpdateMemoryLimitsIncrement > 0.0 && opts.UpdateMemoryLimitsMax <= opts.UpdateMemoryLimitsIncrement {
		fmt.Fprintf(os.Stderr, "Please specify a UpdateMemoryLimitsMax (%.2f) larger than UpdateMemoryLimitsIncrement (%.2f).\n", opts.UpdateMemoryLimitsMax, opts.UpdateMemoryLimitsIncrement)
		os.Exit(1)
	}

	stopChan := make(chan struct{})
	kubeClientResources := util.Clientset() // Create client, dynamic client, RESTMapper...
	reportBuilder := report.NewRightSizerReportBuilder(kubeClientResources,
		report.WithStateConfigMapNamespace(opts.StateConfigMapNamespace),
		report.WithStateConfigMapName(opts.StateConfigMapName),
		report.WithTooOldAge(opts.ResetOOMsWindow),
	)
	eventController := controller.NewController(stopChan,
		kubeClientResources,
		reportBuilder,
		controller.WithAllowedNamespaces(opts.Namespaces),
		controller.WithUpdateMemoryLimits(opts.UpdateMemoryLimits),
		controller.WithUpdateMemoryLimitsMinimumOOMs(opts.UpdateMemoryLimitsMinimumOOMs),
		controller.WithUpdateMemoryLimitsIncrement(opts.UpdateMemoryLimitsIncrement),
		controller.WithUpdateMemoryLimitsMax(opts.UpdateMemoryLimitsMax),
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
