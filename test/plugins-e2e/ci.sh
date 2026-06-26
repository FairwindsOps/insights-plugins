set -xeo pipefail
cd /workspace

memory_check() {
  echo "MEMORY before: $*"
  free -h
  grep -E '^(MemTotal|MemFree|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo
  echo "TOP MEMORY PROCESSES (RSS kB)"
  ps -o pid=,rss=,comm= | sort -k2 -nr | head -10
}

echo "SETTING ENV"
. /workspace/env.sh

memory_check "apk add python3"
apk add python3

memory_check "pip3 install --upgrade pip"
pip3 install --upgrade pip --no-cache-dir

memory_check "pip3 install check-jsonschema"
pip3 install check-jsonschema --no-cache-dir

memory_check "test/plugins-e2e/test.sh"
bash ./test/plugins-e2e/test.sh
