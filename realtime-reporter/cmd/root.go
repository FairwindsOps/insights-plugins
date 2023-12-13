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
	configPath   string
	organization string
	cluster      string
	host         string
	logLevel     string
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
			cwd, err := os.Getwd()
			cobra.CheckErr(err)

			// search config in current directory with name "insights-reporter" (without extension)
			viper.AddConfigPath(cwd + "/examples")
			viper.SetConfigType("yaml")
			viper.SetConfigName("insights-reporter")
		}

		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			logrus.Info("Using config file:", viper.ConfigFileUsed())
			viper.BindPFlag("organization", cmd.Root().Flags().Lookup("organization"))
			viper.BindPFlag("cluster", cmd.Root().Flags().Lookup("cluster"))
			viper.BindPFlag("host", cmd.Root().Flags().Lookup("host"))
			viper.BindEnv("token", "FAIRWINDS_TOKEN")
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
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Location of watch configuration file. Contains a list of resources to watch with an optional field to specify namespaces.")
	rootCmd.MarkFlagRequired("config")
	rootCmd.PersistentFlags().StringVar(&organization, "organization", "", "The Insights organization name.")
	rootCmd.MarkFlagRequired("organization")
	rootCmd.PersistentFlags().StringVar(&cluster, "cluster", "", "The Insights cluster name.")
	rootCmd.MarkFlagRequired("cluster")
	rootCmd.PersistentFlags().StringVar(&host, "host", "https://insights.fairwinds.com", "The Insights host.")
	rootCmd.MarkFlagRequired("host")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", logrus.InfoLevel.String(), "Logrus log level to be output (trace, debug, info, warning, error, fatal, panic).")
}
