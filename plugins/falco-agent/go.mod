module github.com/fairwindsops/insights-plugins/falco-agent

go 1.16

require (
	github.com/fairwindsops/controller-utils v0.1.0
	github.com/falcosecurity/falcosidekick v0.0.0-20211008213138-5ee27aa10373
	github.com/gorilla/mux v1.8.0
	github.com/sirupsen/logrus v1.8.1
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.2
)
