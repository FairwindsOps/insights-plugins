set -xeo pipefail
echo "RUNNING PRE SCRIPT"
mkdir output
echo "export CI_BRANCH='$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')'" >> env.sh
./.circleci/set_tags.sh
echo "DONE SETTING TAGS"
docker cp . e2e-command-runner:/workspace
echo "DONE WITH DOCKER CP"
