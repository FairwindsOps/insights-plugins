package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveTokensAndPassword(t *testing.T) {
	assert.Equal(t, "https://x-access-token:<TOKEN>@github.com/FairwindsOps/charts", RemoveTokensAndPassword("https://x-access-token:my-secret-token@github.com/FairwindsOps/charts"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-creds <CREDENTIALS> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-creds username:password docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-registry-token <TOKEN> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-registry-token my-secret-token docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-creds <CREDENTIALS> docker://us-east1-docker.pkg.dev/gcp-prime/trivy-test/busybox:1.35 docker-archive:./output/tmp/us_east1_docker_pkg_dev_gcp_prime_trivy_test_busybox_1_35", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-creds oauth2accesstoken:what-a-great-oauth2-token --override-arch amd64 --override-os linux docker://us-east1-docker.pkg.dev/gcp-prime/trivy-test/busybox:1.35 docker-archive:./output/tmp/us_east1_docker_pkg_dev_gcp_prime_trivy_test_busybox_1_35"))
}
