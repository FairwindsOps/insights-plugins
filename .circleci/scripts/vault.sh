#! /bin/bash
apt-get update

if ! hash yq; then
  curl -L "https://github.com/mikefarah/yq/releases/download/v4.30.6/yq_linux_amd64" > yq
  chmod +x yq
  mv yq /usr/local/bin/
fi

if ! hash curl; then
  apt-get install -y curl
fi

if ! hash unzip; then
  apt-get install -y unzip
fi

cd /tmp
curl -LO https://releases.hashicorp.com/vault/1.9.2/vault_1.9.2_linux_amd64.zip
unzip vault_1.9.2_linux_amd64.zip

if hash sudo; then
  sudo mv vault /usr/local/bin/vault
else
  mv vault /usr/local/bin/vault
fi
