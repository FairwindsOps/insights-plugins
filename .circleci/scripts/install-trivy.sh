#! /bin/bash
set -eo pipefail

curl -L https://github.com/aquasecurity/trivy/releases/download/v0.23.0/trivy_0.23.0_Linux-64bit.tar.gz > trivy.tar.gz
tar -xvf trivy.tar.gz
mv ./trivy /usr/local/bin/trivy
rm trivy.tar.gz
