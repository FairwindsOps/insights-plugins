set -xeo pipefail
cd /workspace

memory_check() {
  echo "MEMORY before: $*"
  free -h
  grep -E '^(MemTotal|MemFree|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo
  for limit in /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory/memory.limit_in_bytes; do
    if [ -f "$limit" ]; then
      echo "cgroup limit ($limit): $(cat "$limit")"
    fi
  done
  echo "TOP MEMORY PROCESSES (RSS kB)"
  ps -o pid=,rss=,comm= | sort -k2 -nr | head -10
}

echo "SETTING ENV"
. /workspace/env.sh

memory_check "apk add python3 py3-pip"
apk add --no-cache python3 py3-pip

memory_check "pip3 install check-jsonschema"
PIP_DISABLE_PIP_VERSION_CHECK=1 pip3 install check-jsonschema --no-cache-dir

memory_check "test/plugins-e2e/test.sh"
bash ./test/plugins-e2e/test.sh
