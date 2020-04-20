# Insights Plugins

[![FairwindsOps](https://circleci.com/gh/FairwindsOps/insights-plugins.svg?style=svg)](https://circleci.com/gh/FairwindsOps/insights-plugins)
[![Apache 2.0 license](https://img.shields.io/badge/license-Apache2-brightgreen.svg)](https://opensource.org/licenses/Apache-2.0)

This is a repository with plugins for [Insights](https://insights.fairwinds.com).

These can be installed with the official [Insights Agent Helm chart](https://github.com/FairwindsOps/charts/stable/insights-agent)

Each of these plugins retrieves data from a Kubernetes cluster and sends it to Insights for further analysis. Some of these plugins like `trivy` are wrappers around existing Open Source projects. Others like `workload` are self contained. `uploader` is a special case in that it doesn't have any logic in itself, but runs as a sidecar to handle the logic for uploading data to Insights.

**Want to learn more?** Reach out on [the Slack channel](https://fairwindscommunity.slack.com/messages/fairwinds-insights) ([request invite](https://join.slack.com/t/fairwindscommunity/shared_invite/zt-cxss92z7-YjfnJwpUwlviViBFjYV2gg)), send an email to `opensource@fairwinds.com`, or join us for [office hours on Zoom](https://fairwindscommunity.slack.com/messages/office-hours)


## Repository Layout

* `.circleci` contains the Circle CI configuration.
* `deploy` contains rok8s-scripts configuration for each plugin
* `e2e` contains logic for e2e tests
* The remaining folders are one folder per plugin

## Contributing

PRs welcome! Check out the [Contributing Guidelines](CONTRIBUTING.md) and
[Code of Conduct](CODE_OF_CONDUCT.md) for more information.
