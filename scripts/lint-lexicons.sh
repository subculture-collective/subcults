#!/usr/bin/env bash
set -euo pipefail

readonly INDIGO_VERSION='v0.0.0-20251010013709-6e99221dc240'
output_dir="$(mktemp -d)"
trap 'rm -rf -- "${output_dir}"' EXIT
build="[{\"package\":\"subcultlex\",\"prefix\":\"tv.subcult\",\"outdir\":\"${output_dir}\",\"import\":\"example.invalid/subcultlex\"}]"

go run "github.com/bluesky-social/indigo/cmd/lexgen@${INDIGO_VERSION}" \
  --build "${build}" lexicons/tv/subcult/*.json >/dev/null
test "$(find "${output_dir}" -type f -name '*.go' | wc -l)" -eq 9
