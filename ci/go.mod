module github.com/fairwindsops/insights-plugins/ci

go 1.13

require (
	github.com/fairwindsops/insights-plugins/trivy v0.0.0-00010101000000-000000000000
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/sirupsen/logrus v1.6.0
	github.com/thoas/go-funk v0.6.0
	gopkg.in/yaml.v3 v3.0.0-20200506231410-2ff61e1afc86
)

replace github.com/fairwindsops/insights-plugins/trivy => ../trivy
