set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING PYTHON"
apk add python3
python3 --version
echo "ADDING CHECK-JSONSCHEMA"
pip3 install check-jsonschema==0.23.3
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
