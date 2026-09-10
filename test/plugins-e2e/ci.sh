#!/bin/bash
set -xeo pipefail
cd /workspace
echo "SETTING ENV"
. /workspace/env.sh
echo "ADDING CHECK-JSONSCHEMA"
python3 -m pip install --break-system-packages check-jsonschema
echo "RUNNING TESTS"
bash ./test/plugins-e2e/test.sh
