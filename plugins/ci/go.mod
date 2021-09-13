module github.com/fairwindsops/insights-plugins/ci

go 1.16

replace github.com/fairwindsops/insights-plugins/trivy => ../trivy

replace github.com/fairwindsops/insights-plugins/opa => ../opa

require (
	github.com/fairwindsops/insights-plugins/opa v0.0.0-20200904180341-40eda9118d57
	github.com/fairwindsops/insights-plugins/trivy v0.0.0-20200528180806-f7f94de92325
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/thoas/go-funk v0.9.1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)
