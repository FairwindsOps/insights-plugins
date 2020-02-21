#! /bin/sh
set -e
tmp_dir=/output/tmp
mkdir -p $tmp_dir

./scan

echo "finished scanning images"

mv $tmp_dir/final-report.json /output/trivy.json
