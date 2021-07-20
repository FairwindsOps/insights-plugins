module github.com/fairwindsops/insights-plugins/ci

go 1.13

require (
	github.com/fairwindsops/insights-plugins/opa v0.0.0-00010101000000-000000000000
	github.com/fairwindsops/insights-plugins/trivy v0.0.0-00010101000000-000000000000
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/thoas/go-funk v0.7.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/apimachinery v0.20.2
)

replace github.com/fairwindsops/insights-plugins/trivy => ../trivy

replace github.com/fairwindsops/insights-plugins/opa => ../opa
