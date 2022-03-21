#! /bin/bash
set -eo pipefail

curl -L https://github.com/aquasecurity/trivy/releases/download/v0.24.2/trivy_0.24.2_Linux-64bit.tar.gz > trivy.tar.gz
tar -xvf trivy.tar.gz
sudo mv ./trivy /usr/local/bin/trivy
rm trivy.tar.gz
