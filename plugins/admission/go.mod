module github.com/fairwindsops/insights-plugins/admission

go 1.16

replace github.com/fairwindsops/insights-plugins/opa => ../opa

require (
	github.com/fairwindsops/insights-plugins/opa v0.0.0-20200904180341-40eda9118d57
	github.com/fairwindsops/pluto/v3 v3.5.4
	github.com/fairwindsops/polaris v0.0.0-20211026182130-e31f3f1b4156
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/thoas/go-funk v0.9.1
	k8s.io/apimachinery v0.22.2
	sigs.k8s.io/controller-runtime v0.10.2
)
