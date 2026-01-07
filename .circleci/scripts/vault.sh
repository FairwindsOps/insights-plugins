#! /bin/bash

if hash sudo; then
  SUDO=sudo
else
  SUDO=""
fi

$SUDO apt-get update

if ! hash yq; then
  curl -L "https://github.com/mikefarah/yq/releases/download/v4.30.6/yq_linux_amd64" > yq
  chmod +x yq
  $SUDO mv yq /usr/local/bin/
fi

if ! hash curl; then
  $SUDO apt-get install -y curl
fi

if ! hash unzip; then
  $SUDO apt-get install -y unzip
fi

cd /tmp
curl -LO https://releases.hashicorp.com/vault/1.9.2/vault_1.9.2_linux_amd64.zip
unzip vault_1.9.2_linux_amd64.zip

$SUDO mv vault /usr/local/bin/vault
