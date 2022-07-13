#! /bin/bash
set -eo pipefail
insights_cli_version='1.0.2'

echo "Installing insights-cli ${insights_cli_version}"
curl -L https://github.com/FairwindsOps/insights-cli/releases/download/v${insights_cli_version}/insights-cli_${insights_cli_version}_linux_amd64.tar.gz > insights-cli.tar.gz
tar -xzf insights-cli.tar.gz insights-cli
sudo mv ./insights-cli /usr/local/bin/insights-cli
rm insights-cli.tar.gz
