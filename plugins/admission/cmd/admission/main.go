package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	k8sConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	admissionversion "github.com/fairwindsops/insights-plugins/plugins/admission"
	fadmission "github.com/fairwindsops/insights-plugins/plugins/admission/pkg/admission"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	opaversion "github.com/fairwindsops/insights-plugins/plugins/opa"
)

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

func refreshConfig(cfg models.InsightsConfig, handler *fadmission.Validator, mutatorHandler *fadmission.Mutator) error {
	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/admission/configuration?includeRegoV1=true", cfg.Hostname, cfg.Organization, cfg.Cluster)
	logrus.Infof("Refreshing configuration from url %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d - %s", resp.StatusCode, string(body))
	}
	var tempConfig models.Configuration
	err = json.Unmarshal(body, &tempConfig)
	if err != nil {
		return err
	}
	if tempConfig.Polaris == nil {
		logrus.Infoln("no admission polaris config is present in Insights, using the polaris  + config from values.yaml")
		configFromValuesPath := ""
		if _, err := os.Stat("/opt/app/polaris-config.yaml"); err == nil {
			configFromValuesPath = "/opt/app/polaris-config.yaml"
		} else {
			logrus.Infoln("no polaris config from values.yaml found, using default config")
		}
		polarisConfig, err := polarisconfiguration.MergeConfigAndParseFile(configFromValuesPath, true)
		if err != nil {
			return err
		}
		tempConfig.Polaris = &polarisConfig
	}
	logrus.Debugf("The config for Polaris is: %#v", tempConfig.Polaris)
	handler.InjectConfig(tempConfig)
	mutatorHandler.InjectConfig(tempConfig)
	return nil
}

func keepConfigurationRefreshed(ctx context.Context, cfg models.InsightsConfig, interval int, validatorHandler *fadmission.Validator, mutatorHandler *fadmission.Mutator) {
	err := refreshConfig(cfg, validatorHandler, mutatorHandler)
	if err != nil {
		exitWithError("Error refreshing configuration", err)
	}
	ticker := time.NewTicker(time.Minute * time.Duration(interval))
	for {
		select {
		case <-ticker.C:
			err = refreshConfig(cfg, validatorHandler, mutatorHandler)
			if err != nil {
				logrus.Errorf("Error refreshing configuration: %+v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	setLogLevel()
	interval, err := getIntervalOrDefault(1)
	if err != nil {
		exitWithError("could not get interval", err)
	}
	k8sCfg := k8sConfig.GetConfigOrDie()
	iConfig := mustGetInsightsConfigFromEnvVars()
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		exitWithError("could not get k8s clientset from config", err)
	}
	handler := fadmission.NewValidator(clientset, iConfig)
	var mutatorHandler fadmission.Mutator
	go keepConfigurationRefreshed(context.Background(), iConfig, interval, handler, &mutatorHandler)

	webhookPort := int64(8443)
	portString := strings.TrimSpace(os.Getenv("WEBHOOK_PORT"))
	if portString != "" {
		var err error
		webhookPort, err = strconv.ParseInt(portString, 10, 0)
		if err != nil {
			exitWithError("could not parse WEBHOOK_PORT to int", err)
		}
	}

	mgr, err := manager.New(k8sCfg, manager.Options{
		HealthProbeBindAddress: ":8081",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:     int(webhookPort),
			CertDir:  "/opt/cert",
			CertName: "tls.crt",
			KeyName:  "tls.key",
		}),
	})
	if err != nil {
		exitWithError("Unable to set up overall controller manager", err)
	}

	webhookFailurePolicyString := os.Getenv("WEBHOOK_FAILURE_POLICY")
	ok := handler.SetWebhookFailurePolicy(webhookFailurePolicyString)
	if !ok {
		panic(fmt.Sprintf("cannot parse invalid webhook failure policy %q", webhookFailurePolicyString))
	}

	err = mgr.AddReadyzCheck("readyz", healthz.Ping)
	if err != nil {
		exitWithError("Unable to add readyz check", err)
	}
	err = mgr.AddHealthzCheck("healthz", healthz.Ping)
	if err != nil {
		exitWithError("Unable to add healthz check", err)
	}

	_, err = os.Stat("/opt/cert/tls.crt")
	if os.IsNotExist(err) {
		time.Sleep(time.Second * 10)
		panic("Cert does not exist")
	}

	mgr.GetWebhookServer().Register("/validate", &webhook.Admission{Handler: handler})
	mgr.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: &mutatorHandler})

	logrus.Infof("Starting webhook manager %s (OPA %s)", admissionversion.String(), opaversion.String())
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logrus.Errorf("Error starting manager: %v", err)
		os.Exit(1)
	}
}

func getIntervalOrDefault(fallback int) (int, error) {
	durationString := os.Getenv("CONFIGURATION_INTERVAL")
	if durationString == "" {
		return fallback, nil
	}
	durationInt, err := strconv.Atoi(durationString)
	if err != nil {
		return 0, fmt.Errorf("CONFIGURATION_INTERVAL is not an integer: %v", err)
	}
	return durationInt, nil
}

func mustGetInsightsConfigFromEnvVars() models.InsightsConfig {
	hostname := os.Getenv("FAIRWINDS_HOSTNAME")
	if hostname == "" {
		exitWithError("FAIRWINDS_HOSTNAME environment variable not set", nil)
	}
	organization := os.Getenv("FAIRWINDS_ORGANIZATION")
	if organization == "" {
		exitWithError("FAIRWINDS_ORGANIZATION environment variable not set", nil)
	}
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	if cluster == "" {
		exitWithError("FAIRWINDS_CLUSTER environment variable not set", nil)
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		exitWithError("FAIRWINDS_TOKEN environment variable not set", nil)
	}
	usernameTokens := strings.Split(os.Getenv("FAIRWINDS_IGNORE_USERNAMES"), ",")
	ignoreUsernames := []string{}
	for _, username := range usernameTokens {
		ignoreUsernames = append(ignoreUsernames, strings.TrimSpace(username))
	}
	return models.InsightsConfig{Hostname: hostname, Organization: organization, Cluster: cluster, Token: token, IgnoreUsernames: ignoreUsernames}
}

func setLogLevel() {
	if os.Getenv("LOGRUS_LEVEL") != "" {
		lvl, err := logrus.ParseLevel(os.Getenv("LOGRUS_LEVEL"))
		if err != nil {
			panic(err)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
