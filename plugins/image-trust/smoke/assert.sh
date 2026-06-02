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

deployment_status() {
  local deploy="$1"
  jq -r --arg d "${deploy}" '
    [.images[]
      | select(any(.owners[]?; .namespace == "image-trust-smoke" and .name == $d))
    ]
    | if length == 0 then "missing" else .[0].status end
  ' "${REPORT}"
}

deployment_field() {
  local deploy="$1"
  local field="$2"
  jq -r --arg d "${deploy}" --arg f "${field}" '
    [.images[]
      | select(any(.owners[]?; .namespace == "image-trust-smoke" and .name == $d))
    ]
    | if length == 0 then "" else .[0][$f] // "" end
  ' "${REPORT}"
}

expect_deployment() {
  local deploy="$1"
  local want_status="$2"
  local actual
  actual="$(deployment_status "${deploy}")"
  if [[ "${actual}" != "${want_status}" ]]; then
    echo "FAIL: deployment ${deploy} expected status ${want_status}, got ${actual}" >&2
    fail=1
  else
    echo "OK: deployment ${deploy} status = ${actual}"
  fi
}

image_status() {
  local pattern="$1"
  jq -r --arg p "${pattern}" '
    [.images[] | select(.name | contains($p))]
    | if length == 0 then "missing" else .[0].status end
  ' "${REPORT}"
}

expect_image() {
  local pattern="$1"
  local want_status="$2"
  local actual
  actual="$(image_status "${pattern}")"
  if [[ "${actual}" != "${want_status}" ]]; then
    echo "FAIL: image matching ${pattern} expected status ${want_status}, got ${actual}" >&2
    fail=1
  else
    echo "OK: image matching ${pattern} status = ${actual}"
  fi
}

check verified "${verified}" 2
check signedUntrusted "${untrusted}" 1
check unsigned "${unsigned}" 1

expect_image 'cgr.dev/chainguard/busybox' verified
expect_image 'us-docker.pkg.dev/fairwinds-ops/oss/polaris' verified
expect_image 'gcr.io/projectsigstore/cosign' signed_untrusted
expect_image 'docker.io/library/busybox' unsigned

expect_deployment keyed-verified verified

keyed_verified_by="$(deployment_field keyed-verified verifiedBy)"
if [[ "${keyed_verified_by}" != "cosign-key" ]]; then
  echo "FAIL: deployment keyed-verified expected verifiedBy cosign-key, got ${keyed_verified_by:-<empty>}" >&2
  fail=1
else
  echo "OK: deployment keyed-verified verifiedBy = cosign-key"
fi

keyed_key_ref="$(jq -r '
  [.images[]
    | select(any(.owners[]?; .namespace == "image-trust-smoke" and .name == "keyed-verified"))
  ]
  | if length == 0 then "" else .[0].signer.keyRef // "" end
' "${REPORT}")"
if [[ "${keyed_key_ref}" != 'https://artifacts.fairwinds.com/cosign-p256.pub' ]]; then
  echo "FAIL: deployment keyed-verified expected signer.keyRef https://artifacts.fairwinds.com/cosign-p256.pub, got ${keyed_key_ref:-<empty>}" >&2
  fail=1
else
  echo "OK: deployment keyed-verified signer.keyRef = ${keyed_key_ref}"
fi

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
if [[ "${smoke_images}" -lt 4 ]]; then
  echo "FAIL: expected at least 4 images from namespace image-trust-smoke, found ${smoke_images}" >&2
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
