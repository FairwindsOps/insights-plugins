#! /bin/bash
set -eo pipefail

trivyVersion="0.68.0"

curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_Linux-64bit.tar.gz > trivy.tar.gz
tar -xvf trivy.tar.gz
sudo mv ./trivy /usr/local/bin/trivy
rm trivy.tar.gz
