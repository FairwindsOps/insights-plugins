# CI

A utility for the CI/CD integration of Fairwinds Insights.

Visit
[insights.docs.fairwinds.com](https://insights.docs.fairwinds.com/features/continuous-integration/)
for the full documentation


# update trivy and opa

Modify `project` and `sha` accordingly
> go get github.com/fairwindsops/insights-plugins/plugins/${project}@${sha}

# Running command example

## auto-scan
```
GOOS=linux GOARCH=amd64 go build -o insights-ci cmd/insights-ci/main.go && \
docker build . --tag insights-ci:latest && \
docker run -v /Users/vvezani/fairwinds/insights-plugins/plugins/ci/.tmp:/app/repository \
      -e "LOGRUS_LEVEL=debug" \
      -e "CLONE_REPO=true" \
      -e "FAIRWINDS_TOKEN=thisisacitoken" \
      -e "SCRIPT_VERSION=" \
      -e "IMAGE_VERSION=0.0.1" \
      -e "REPOSITORY_NAME=vitorvezani/blog" \
      -e "BRANCH_NAME=main" \
      -e "BASE_BRANCH=main" \
      -e "GITHUB_ACCESS_TOKEN=" \
      -e "ORG_NAME=acme-co" \
      -e "HOSTNAME=https://be-main.k8s.insights.fairwinds.com" \
      -e "LOGRUS_LEVEL=debug" \
      -e 'REGISTRY_CREDENTIALS=[{"domain": "docker.io", "username": "my-user", "password": "my-pass"}]' \
      -e 'REPORTS_CONFIG={"autoScan": {"polaris": {"enabledOnAutoDiscovery": false}, "opa": {"enabledOnAutoDiscovery": false}, "pluto": {"enabledOnAutoDiscovery": false}, "trivy": {"enabledOnAutoDiscovery": false}, "tfsec": {"enabledOnAutoDiscovery": false}}}' \
  insights-ci:latest && \ 
rm -rf ./.tmp/
```