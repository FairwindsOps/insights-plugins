module github.com/fairwindsops/insights-plugins/prometheus

go 1.15

require (
	github.com/fairwindsops/controller-utils v0.1.0
	github.com/imroc/req v0.3.0
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/common v0.14.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/controller-runtime v0.6.3
)
