# CI

A utility for the CI/CD integration of Fairwinds Insights.

Visit
[insights.docs.fairwinds.com](https://insights.docs.fairwinds.com/features/continuous-integration/)
for the full documentation

# Testing

Docker compose is currently configure for testing cloned repos using `docker-compose up --build`.

`docker-compose.yaml` can be configured via the following variables to test your own repo:
  - "CLONE_REPO=true"
  - "FAIRWINDS_TOKEN=thisisacitoken"
  - "SCRIPT_VERSION="
  - "IMAGE_VERSION=build_6444"
  - "REPOSITORY_NAME=vitorvezani/blog"
  - "BRANCH=main"
  - "ACCESS_TOKEN=" // needed for private repositories