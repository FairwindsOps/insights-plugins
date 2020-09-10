package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	"github.com/sirupsen/logrus"
	k8sConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	fadmission "github.com/fairwindsops/insights-plugins/admission/pkg/admission"
	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

var handler fadmission.Validator

var organization string
var hostname string
var cluster string

func refreshConfig() error {
	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/admission/configuration", hostname, organization, cluster)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var tempConfig models.Configuration
	err = json.Unmarshal(body, &tempConfig)
	if err != nil {
		return err
	}
	if tempConfig.Polaris == nil {
		polarisConfig, err := polarisconfiguration.ParseFile("")
		tempConfig.Polaris = &polarisConfig
		if err != nil {
			return err
		}
	}
	logrus.Infof("Config: %+v", tempConfig)
	handler.Config = tempConfig
	return nil
}

func keepConfigurationRefreshed(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 30)
	err := refreshConfig()
	if err != nil {
		exitWithError("Error refreshing configuration", err)
	}
	for {
		select {
		case <-ticker.C:
			err = refreshConfig()
			if err != nil {
				logrus.Errorf("Error refreshing configuration: %+v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	organization = os.Getenv("FAIRWINDS_ORGANIZATION")
	hostname = os.Getenv("FAIRWINDS_HOSTNAME")
	cluster = os.Getenv("FAIRWINDS_CLUSTER")
	var err error
	go keepConfigurationRefreshed(context.Background())

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		exitWithError("FAIRWINDS_TOKEN environment variable not set", nil)
	}
	webhookPort := 8443
	mgr, err := manager.New(k8sConfig.GetConfigOrDie(), manager.Options{
		CertDir: "/opt/cert",
		Port:    webhookPort,
	})
	if err != nil {
		exitWithError("Unable to set up overall controller manager", err)
	}

	_, err = os.Stat("/opt/cert/tls.crt")
	if os.IsNotExist(err) {
		time.Sleep(time.Second * 10)
		panic("Cert does not exist")
	}
	server := mgr.GetWebhookServer()
	server.CertName = "tls.crt"
	server.KeyName = "tls.key"

	mgr.GetWebhookServer().Register("/validate", &webhook.Admission{Handler: &handler})

	logrus.Debug("Starting webhook manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logrus.Errorf("Error starting manager: %v", err)
		os.Exit(1)
	}

}
