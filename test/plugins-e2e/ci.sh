set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING PYTHON"
apk add --no-cache python3 py3-pip
echo "ADDING CHECK-JSONSCHEMA"
PIP_NO_CACHE_DIR=1 pip3 install --no-cache-dir check-jsonschema
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
