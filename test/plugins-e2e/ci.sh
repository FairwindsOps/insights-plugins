set -eo pipefail
cd /workspace
. /workspace/env.sh
apk add python3
pip3 install check-jsonschema
bash ./test/plugins-e2e/test.sh