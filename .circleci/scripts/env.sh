set -eo pipefail
sudo_cmd=""
if [ "$(id -u)" != "0" ] ; then
  # This script may be run as a non-root user, which requires sudo
sudo_cmd="sudo"
fi

${sudo_cmd} apk update && ${sudo_cmd} apk add ca-certificates
${sudo_cmd} update-ca-certificates

echo 'export SLACK_DEFAULT_CHANNEL=CPYBX810T' >> ${BASH_ENV}
echo 'export GO111MODULE=on' >> ${BASH_ENV}
echo 'export CI_SHA1=$CIRCLE_SHA1' >> ${BASH_ENV}
echo 'export CI_BRANCH=$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')' >> ${BASH_ENV}
echo 'export CI_BUILD_NUM=$CIRCLE_BUILD_NUM' >> ${BASH_ENV}
echo 'export CI_TAG=$(echo "${CIRCLE_TAG:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')' >> ${BASH_ENV}
echo 'export AWS_DEFAULT_REGION=us-east-1' >> ${BASH_ENV}
echo 'export GOPROXY=https://proxy.golang.org' >> ${BASH_ENV}
