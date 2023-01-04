package main

import (
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/falco-agent/pkg/server"
)

const port = 3031

func main() {
	s, err := server.CreateServer(port)
	if err != nil {
		panic(err)
	}
	logrus.Infof("starting server at http://0.0.0.0:%d", port)
	logrus.Fatal(s.ListenAndServe())
}
