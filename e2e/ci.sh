set -eo pipefail
cd /workspace
. /workspace/env.sh
apk add python3
bash ./e2e/test.sh