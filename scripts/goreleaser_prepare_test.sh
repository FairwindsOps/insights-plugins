#!/bin/sh
# Checks prepare_goreleaser_config: immutable GAR arch tags are removed from
# the buildx push only, and each arch image is built without provenance.
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
- name_template: us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .Tag }}
  skip_push: ${skip_gar_version_tag}
  image_templates:
  - "us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64"
'

dockers_of() {
  printf '%s\n' "$1" | awk '/^docker_manifests:/{exit} /^dockers:/{p=1} p'
}

manifests_of() {
  printf '%s\n' "$1" | awk '/^docker_manifests:/{p=1} p'
}

assert_retry() {
  out=$(printf '%s\n' "$fixture" | sh "$script" --prepare-config true true true)
  dockers=$(dockers_of "$out")
  manifests=$(manifests_of "$out")
  printf '%s\n' "$dockers" | grep -q 'quay.io/fairwinds/kyverno:{{ .FullCommit }}-amd64' || fail "retry dropped the quay amd64 tag"
  printf '%s\n' "$dockers" | grep -q 'quay.io/fairwinds/kyverno:{{ .FullCommit }}-arm64' || fail "retry dropped the quay arm64 tag"
  if printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev'; then
    fail "retry still pushes a GAR arch tag"
  fi
  printf '%s\n' "$dockers" | grep -q 'IG_VERSION' || fail "retry dropped other build flags"
  printf '%s\n' "$manifests" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64' || fail "retry dropped the GAR manifest image ref"
  printf '%s\n' "$manifests" | grep -q 'skip_push: ${skip_gar_version_tag}' || fail "retry changed the version-tag skip"
  commit_skip=$(printf '%s\n' "$manifests" | awk '
    /name_template: us-docker\.pkg\.dev\/fairwinds-ops\/oss\/kyverno:\{\{ \.FullCommit \}\}/ { print; getline; print }
  ')
  printf '%s\n' "$commit_skip" | grep -q 'skip_push: true' || fail "retry still publishes the GAR commit manifest"
  quay_skip=$(printf '%s\n' "$manifests" | awk '
    /name_template: quay.io\/fairwinds\/kyverno:\{\{ \.FullCommit \}\}/ { print; getline; print }
  ')
  if printf '%s\n' "$quay_skip" | grep -q 'skip_push:'; then
    fail "retry skipped the quay commit manifest"
  fi
  n=$(printf '%s\n' "$dockers" | grep -c -- '--provenance=false' || true)
  [ "$n" = 2 ] || fail "retry provenance flags: expected 2, got ${n}"
  n=$(printf '%s\n' "$dockers" | grep -c -- '--sbom=false' || true)
  [ "$n" = 2 ] || fail "retry sbom flags: expected 2, got ${n}"
}

assert_first_push() {
  out=$(printf '%s\n' "$fixture" | sh "$script" --prepare-config false false false)
  dockers=$(dockers_of "$out")
  manifests=$(manifests_of "$out")
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64' || fail "first push dropped the GAR amd64 tag"
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64' || fail "first push dropped the GAR arm64 tag"
  if printf '%s\n' "$manifests" | grep -q 'skip_push: true'; then
    fail "first push skipped the GAR commit manifest"
  fi
  n=$(printf '%s\n' "$dockers" | grep -c -- '--provenance=false' || true)
  [ "$n" = 2 ] || fail "first push provenance flags: expected 2, got ${n}"
}

assert_one_arch() {
  out=$(printf '%s\n' "$fixture" | sh "$script" --prepare-config true false false)
  dockers=$(dockers_of "$out")
  if printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-amd64'; then
    fail "partial retry still pushes the published amd64 tag"
  fi
  printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev/fairwinds-ops/oss/kyverno:{{ .FullCommit }}-arm64' || fail "partial retry dropped the missing arm64 tag"
}

assert_plugins() {
  for config in "${root}"/plugins/*/.goreleaser.yml.envsubst; do
    out=$(sh "$script" --prepare-config true false false < "$config")
    platforms=$(printf '%s\n' "$out" | grep -c -- '--platform=' || true)
    provenance=$(printf '%s\n' "$out" | grep -c -- '--provenance=false' || true)
    [ "$platforms" = "$provenance" ] || fail "${config}: provenance ${provenance} != platforms ${platforms}"
    dockers=$(dockers_of "$out")
    if printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev.*-amd64'; then
      fail "${config}: published amd64 tag still in dockers"
    fi
    printf '%s\n' "$dockers" | grep -q 'us-docker.pkg.dev.*-arm64' || fail "${config}: missing arm64 tag was removed"
  done
}

assert_retry
assert_first_push
assert_one_arch
assert_plugins
echo "ok"
