package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/watcher"
)

var (
	configPath string
	logLevel   string
)

var rootCmd = &cobra.Command{
	Use:   "insights-reporter",
	Short: "A generator for Cobra based Applications",
	Long: `Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		parsedLevel, err := logrus.ParseLevel(logLevel)
		if err != nil {
			logrus.Errorf("log-level flag has invalid value %s", logLevel)
		} else {
			logrus.SetLevel(parsedLevel)
		}

		if configPath != "" {
			// use config file from the flag.
			viper.SetConfigFile(configPath)
		} else {
			// find home directory
			home, err := os.UserHomeDir()
			cobra.CheckErr(err)

			// search config in home directory with name ".insights-reporter" (without extension)
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".insights-reporter")
		}

		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			logrus.Info("Using config file:", viper.ConfigFileUsed())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		watcher, err := watcher.NewWatcher()
		if err != nil {
			logrus.Fatalf("Error creating new watcher: %s", err.Error())
		}

		stopCh := make(chan struct{})
		defer close(stopCh)

		watcher.Run(stopCh)

		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGQUIT, syscall.SIGSTOP)

		<-sigterm
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Location of configuration file.")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", logrus.InfoLevel.String(), "Logrus log level to be output (trace, debug, info, warning, error, fatal, panic).")
}
