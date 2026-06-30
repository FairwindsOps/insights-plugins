set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING PYTHON AND CHECK-JSONSCHEMA"
apk add --no-cache python3 check-jsonschema
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
