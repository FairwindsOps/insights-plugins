#!/bin/sh
# Published GAR arch tags are removed from the buildx push only.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
script="${root}/scripts/goreleaser.sh"
fail() { echo "FAIL: $1" >&2; exit 1; }

fixture='dockers:
- image_templates:
  - "quay.io/fairwinds/kyverno:{{ .FullCommit }}-amd64"
  - "us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64"
  build_flag_templates:
  - "--platform=linux/amd64"
  - "--build-arg=IG_VERSION={{ .Env.IG_VERSION }}"
- image_templates:
  - "quay.io/fairwinds/kyverno:{{ .FullCommit }}-arm64"
  - "us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64"
  build_flag_templates:
  - "--platform=linux/arm64"
docker_manifests:
- name_template: quay.io/fairwinds/kyverno:{{ .FullCommit }}
  image_templates:
  - "quay.io/fairwinds/kyverno:{{ .FullCommit }}-amd64"
- name_template: us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}
  image_templates:
  - "us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64"
  - "us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64"
'

dockers_of() {
  printf '%s\n' "$1" | awk '/^docker_manifests:/{exit} /^dockers:/{p=1} p'
}

manifests_of() {
  printf '%s\n' "$1" | awk '/^docker_manifests:/{p=1} p'
}

run() {
  printf '%s\n' "$fixture" | sh "$script" --prepare-config "$1" "$2"
}

assert_retry() {
  out=$(run true true)
  dockers=$(dockers_of "$out")
  manifests=$(manifests_of "$out")
  printf '%s\n' "$dockers" | grep -q 'quay.io/fairwinds/kyverno:{{ .FullCommit }}-amd64' || fail "retry dropped the quay amd64 tag"
  printf '%s\n' "$dockers" | grep -q 'quay.io/fairwinds/kyverno:{{ .FullCommit }}-arm64' || fail "retry dropped the quay arm64 tag"
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev' && fail "retry still pushes a GAR arch tag"
  printf '%s\n' "$dockers" | grep -q 'IG_VERSION' || fail "retry dropped other build flags"
  printf '%s\n' "$manifests" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64' || fail "retry dropped the GAR manifest image ref"
}

assert_first_push() {
  out=$(run false false)
  dockers=$(dockers_of "$out")
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64' || fail "first push dropped the GAR amd64 tag"
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64' || fail "first push dropped the GAR arm64 tag"
}

assert_one_arch() {
  dockers=$(dockers_of "$(run true false)")
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64' && fail "partial retry still pushes the published amd64 tag"
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64' || fail "partial retry dropped the missing arm64 tag"
}

assert_plugins() {
  for config in "${root}"/plugins/*/.goreleaser.yml.envsubst; do
    grep -q 'skip_push: ${skip_gar_commit_tag}' "$config" || fail "${config}: missing skip_gar_commit_tag"
    out=$(sh "$script" --prepare-config true false < "$config")
    dockers=$(dockers_of "$out")
    manifests=$(manifests_of "$out")
    printf '%s\n' "$dockers" | grep -q 'quay.io' || fail "${config}: dropped quay"
    printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev.*-amd64' && fail "${config}: published amd64 tag still in dockers"
    printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev.*-arm64' || fail "${config}: missing arm64 tag was removed"
    printf '%s\n' "$manifests" | grep -q 'us-docker.pkg.dev.*-amd64' || fail "${config}: dropped the GAR manifest ref"
  done
}

assert_retry
assert_first_push
assert_one_arch
assert_plugins
echo "ok"
