set -xeo pipefail
mkdir output
echo "export CI_BRANCH='$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')'" >> env.sh
docker cp . e2e-command-runner:/workspace
docker cp tags.sh e2e-command-runner:/workspace/tags.sh
