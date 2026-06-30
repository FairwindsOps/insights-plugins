set -xeo pipefail
echo "RUNNING PRE SCRIPT"

# ci-images alpine uses docker:dind as its entrypoint; the orb starts the command
# runner without overriding it, so the container exits when dind cannot mount
# /sys/kernel/security. Keep it alive with sleep so docker exec can run ci.sh.
echo "RECREATING COMMAND RUNNER"
docker rm -f e2e-command-runner || true
docker run -d \
  --name e2e-command-runner \
  --network container:e2e-control-plane \
  --env KUBECONFIG=/.kube/config \
  --env CI_SHA1="${CIRCLE_SHA1:-}" \
  --env CI_BRANCH="${CIRCLE_BRANCH:-}" \
  --env CI_BUILD_NUM="${CIRCLE_BUILD_NUM:-}" \
  --env CI_TAG="${CIRCLE_TAG:-}" \
  --entrypoint sleep \
  quay.io/reactiveops/ci-images:v15.0-alpine \
  infinity
docker exec e2e-command-runner sh -c 'mkdir -p /.kube'
docker cp ./kind-kubeconfig e2e-command-runner:/.kube/config
docker exec e2e-command-runner sh -c "sed -i 's/https:\/\/127.0.0.1:[0-9]\+/https:\/\/127.0.0.1:6443/g' /.kube/config"
docker exec e2e-command-runner kubectl get nodes

mkdir output
echo "export CI_BRANCH='$(echo "${CIRCLE_BRANCH:0:26}" | sed 's/[^a-zA-Z0-9]/-/g' | sed 's/-\+$//')'" >> env.sh
./.circleci/set_tags.sh
echo "DONE SETTING TAGS"
docker cp . e2e-command-runner:/workspace
echo "DONE WITH DOCKER CP"
