set -eo pipefail
cd /workspace
. /env.sh
apk add bash
bash ./e2e/test.sh