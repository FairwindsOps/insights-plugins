module github.com/fairwindsops/insights-plugins/prometheus

go 1.15

require (
	github.com/fairwindsops/controller-utils v0.1.0
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/common v0.14.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.6.3
)
