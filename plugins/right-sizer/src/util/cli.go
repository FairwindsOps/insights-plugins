package util

import (
	"flag"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	flags "github.com/jessevdk/go-flags"
)

// ParseArgs needs a struct compatible to jeddevdk/go-flags and will fill it
// based on CLI parameters.
func ParseArgs(options any) {
	_, err := flags.ParseArgs(options, os.Args)
	if err != nil {
		if err.(*flags.Error).Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	fixGlog(options)
}

// InstallSignalHandler sends a message on sigint or sigterm
func InstallSignalHandler(stop chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		stop <- struct{}{}
		close(stop)
	}()
}

// configure glog, not used for flag parsing
func fixGlog(options any) {
	flag.Set("logtostderr", "true")

	verbose := reflect.ValueOf(options).Elem().FieldByName("Verbose")
	if verbose.IsValid() {
		flag.Set("v", strconv.Itoa(verbose.Interface().(int)))
	}
	// glog cannot filter Info/Warning/Error calls, so LOG_LEVEL only raises verbosity.
	if v := flag.Lookup("v"); v != nil && v.Value.String() == "0" && strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		flag.Set("v", "2")
	}
	flag.CommandLine.Parse([]string{})
}
