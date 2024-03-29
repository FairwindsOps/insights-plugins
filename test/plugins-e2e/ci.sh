set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING PYTHON"
apk add python3
echo "UPDATING PIP"
pip3 install --upgrade pip
echo "ADDING CHECK-JSONSCHEMA"
pip3 install check-jsonschema
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
