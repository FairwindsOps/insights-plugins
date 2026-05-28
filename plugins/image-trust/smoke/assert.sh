#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORT="${ROOT}/../output/image-trust.json"

if [[ ! -f "${REPORT}" ]]; then
  echo "report not found: ${REPORT} (run ./run.sh first)" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

echo "Summary:"
jq '.summary' "${REPORT}"

verified="$(jq -r '.summary.verified // 0' "${REPORT}")"
untrusted="$(jq -r '.summary.signedUntrusted // 0' "${REPORT}")"
unsigned="$(jq -r '.summary.unsigned // 0' "${REPORT}")"
errors="$(jq -r '.summary.verificationError // 0' "${REPORT}")"

fail=0
check() {
  local name="$1"
  local actual="$2"
  local want="$3"
  if [[ "${actual}" -lt "${want}" ]]; then
    echo "FAIL: expected summary.${name} >= ${want}, got ${actual}" >&2
    fail=1
  else
    echo "OK: summary.${name} = ${actual} (>= ${want})"
  fi
}

check verified "${verified}" 1
check signedUntrusted "${untrusted}" 1
check unsigned "${unsigned}" 1

if [[ "${errors}" -ne 0 ]]; then
  echo "FAIL: expected summary.verificationError = 0, got ${errors}" >&2
  fail=1
else
  echo "OK: summary.verificationError = 0"
fi

echo ""
echo ""
echo "Per-image status:"
jq -r '.images[] | "\(.status)\t\(.name)"' "${REPORT}" | sort

smoke_images="$(jq '[.images[] | select(any(.owners[]?; .namespace == "image-trust-smoke"))] | length' "${REPORT}")"
if [[ "${smoke_images}" -lt 3 ]]; then
  echo "FAIL: expected at least 3 images from namespace image-trust-smoke, found ${smoke_images}" >&2
  echo "Run ./setup.sh then ./run.sh (run.sh auto-runs setup when the namespace is empty)." >&2
  fail=1
fi

if [[ "${fail}" -ne 0 ]]; then
  echo ""
  echo "ActionItems:"
  jq '.ActionItems' "${REPORT}"
  exit 1
fi

echo ""
echo "Smoke test passed."
