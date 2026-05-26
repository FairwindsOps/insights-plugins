#! /bin/sh
set -e

tmp_dir=/output/tmp
mkdir -p "$tmp_dir"

image-trust

echo "finished generating image trust report"

mv "$tmp_dir/final-report.json" /output/image-trust.json
