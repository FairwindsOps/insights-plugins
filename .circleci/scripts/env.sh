set -eo pipefail

if [ -x "$(command -v sudo)" ]; then
  sudo apt-get update
  sudo apt-get install apt-transport-https ca-certificates libssl-dev -y
  sudo update-ca-certificates
else
  apt-get update
  apt-get install apt-transport-https ca-certificates libssl-dev -y
  update-ca-certificates
fi
echo 'export GO111MODULE=on' >> ${BASH_ENV}
echo 'export CI_SHA1=$CIRCLE_SHA1' >> ${BASH_ENV}
echo 'export CI_BRANCH=$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')' >> ${BASH_ENV}
echo 'export CI_BUILD_NUM=$CIRCLE_BUILD_NUM' >> ${BASH_ENV}
echo 'export CI_TAG=$(echo "${CIRCLE_TAG:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')' >> ${BASH_ENV}
echo 'export AWS_DEFAULT_REGION=us-east-1' >> ${BASH_ENV}
echo 'export GOPROXY=https://proxy.golang.org' >> ${BASH_ENV}

