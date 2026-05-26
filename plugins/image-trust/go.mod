module github.com/fairwindsops/insights-plugins/plugins/image-trust

go 1.26.2

require (
	github.com/fairwindsops/controller-utils v0.3.4
	github.com/samber/lo v1.53.0
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.11.1
	k8s.io/api v0.36.1
	k8s.io/apimachinery v0.36.1
	k8s.io/client-go v0.36.1
	sigs.k8s.io/controller-runtime v0.24.1
)

require github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
