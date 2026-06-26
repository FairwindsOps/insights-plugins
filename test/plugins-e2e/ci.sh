set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING PYTHON"

free -h
cat /proc/meminfo | head -20
ps aux --sort=-rss | head -20

apk add python3
echo "UPDATING PIP"
pip3 install --upgrade pip --no-cache-dir
echo "ADDING CHECK-JSONSCHEMA"
pip3 install check-jsonschema --no-cache-dir
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
