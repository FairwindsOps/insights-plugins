set -eo pipefail
cd /workspace
. /workspace/env.sh
apk add python3
pip3 install jsonschema
bash ./e2e/test.sh