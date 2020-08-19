module github.com/fairwindsops/insights-plugins/admission

go 1.13

require (
	github.com/fairwindsops/insights-plugins/opa v0.0.0-20200901152700-f5c1d3d67036
	github.com/fairwindsops/pluto v1.1.0
	github.com/fairwindsops/pluto/v3 v3.4.1
	github.com/fairwindsops/polaris v0.0.0-20200826172321-7a0efb785352
	github.com/sirupsen/logrus v1.6.0
	github.com/thoas/go-funk v0.7.0
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	sigs.k8s.io/controller-runtime v0.6.1
)

replace github.com/fairwindsops/insights-plugins/opa => ../opa
