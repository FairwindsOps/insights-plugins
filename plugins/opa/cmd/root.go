/*
Copyright © 2022 FairwindsOps Inc
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	opa "github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "opa",
	Short: "Fairwinds Insights OPA plugin",
	Long: `The OPA plugin runs custom OPA policies against existing Kubernetes resources, to create Action Items.
	See the Fairwinds Insights documentation, at https://insights.docs.fairwinds.com/reports/opa/`,
	Run: func(cmd *cobra.Command, args []string) {
		startOPAReporter()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.opa.yaml)")
	rootCmd.PersistentFlags().BoolP("debug", "D", false, "Enable debug logging.")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	rootCmd.PersistentFlags().StringArrayP("target-resource", "r", []string{}, "A Kubernetes target specified as APIGroup[,APIGroup...]/Resource[,Resource...]. For example: apps/Deployments,Daemonsets")
	viper.BindPFlag("target-resource", rootCmd.PersistentFlags().Lookup("target-resource"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".opa" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".opa")
	}
	viper.AutomaticEnv() // read in environment variables that match
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

const outputFile = "/output/opa.json"
const outputTempFile = "/output/opa-temp.json"

// Output is the format for the output file
type Output struct {
	ActionItems []opa.ActionItem
}

func startOPAReporter() {
	if viper.GetBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debugf("Debugging is enabled...")
	}
	opa.CLIKubeTargets = opa.ProcessCLIKubeResourceTargets(viper.GetStringSlice("target-resource"))
	logrus.Info("Starting OPA reporter")
	ctx := context.Background()
	actionItems, runError := opa.Run(ctx)
	if actionItems != nil {
		logrus.Infof("Finished processing OPA checks, found %d Action Items", len(actionItems))

		output := Output{ActionItems: actionItems}
		value, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(outputTempFile, value, 0644)
		if err != nil {
			panic(err)
		}
		err = os.Rename(outputTempFile, outputFile)
		if err != nil {
			panic(err)
		}
	}
	if runError != nil {
		logrus.Error("There were errors while processing OPA checks.")
		fmt.Fprintln(os.Stderr, runError)
		os.Exit(1)
	}
}
