module github.com/fairwindsops/insights-plugins/admission

go 1.16

replace github.com/fairwindsops/insights-plugins/opa => ../opa

require (
	github.com/fairwindsops/insights-plugins/opa v0.0.0-20200904180341-40eda9118d57
	github.com/fairwindsops/pluto/v3 v3.5.4
	github.com/fairwindsops/polaris v0.0.0-20211103123229-268f0e681111
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/thoas/go-funk v0.9.1
	k8s.io/apimachinery v0.22.3
	sigs.k8s.io/controller-runtime v0.10.2
)
