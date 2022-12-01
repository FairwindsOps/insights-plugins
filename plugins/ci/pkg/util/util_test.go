package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveTokensAndPassword(t *testing.T) {
	assert.Equal(t, "https://x-access-token:<TOKEN>@github.com/FairwindsOps/charts", RemoveTokensAndPassword("https://x-access-token:my-secret-token@github.com/FairwindsOps/charts"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-creds <CREDENTIALS> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-creds username:password docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
	assert.Equal(t, "/usr/bin/skopeo copy --src-registry-token <TOKEN> docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130", RemoveTokensAndPassword("/usr/bin/skopeo copy --src-registry-token my-secret-token docker://alpine:3.13.0 docker-archive:/app/repository/tmp/_insightsTempImages/alpine3130"))
}
