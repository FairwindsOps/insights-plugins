# Contributing

Issues, whether bugs, tasks, or feature requests are essential for improving this project. We believe it should be as easy as possible to contribute changes that get things working in your environment. There are a few guidelines that we need contributors to follow so that we can keep on top of things.

## Code of Conduct

This project adheres to a [code of conduct](CODE_OF_CONDUCT.md). Please review this document before contributing to this project.

## Sign the CLA

Before you can contribute, you will need to sign the [Contributor License Agreement](https://cla-assistant.io/fairwindsops/insights-plugins).

## Project Structure

Insights Plugins is a collection of loosely coupled image builds that are used together for the Insights Agent.

## Getting Started

We label issues with the ["good first issue" tag](https://github.com/FairwindsOps/insights-plugins/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) if we believe they'll be a good starting point for new contributors. If you're interested in working on an issue, please start a conversation on that issue, and we can help answer any questions as they come up. Another good place to start would be adding regression tests to existing plugins.

## Setting Up Your Development Environment
### Prerequisites
* A local install of Docker to test builds
* A local Kubernetes cluster is very helpful, [KIND](https://github.com/kubernetes-sigs/kind) can be used.

## Running Tests

The tests are currently not runnable locally.

## Creating a New Issue

If you've encountered an issue that is not already reported, please create a [new issue](https://github.com/FairwindsOps/insights-plugins/issues), choose `Bug Report`, `Feature Request` or `Misc.` and follow the instructions in the template. 


## Creating a Pull Request

Each new pull request should:

- Reference any related issues
- Pass existing tests and linting
- Contain a clear indication of if they're ready for review or a work in progress
- Be up to date and/or rebased on the master branch

## Creating a new release

### Patch releases
Patch releases only need to change this repo. The Helm chart and deploy scripts
will automatically pull in the latest changes.

If there are any breaking changes then make this a minor or major version increase.

1. Create a PR for this repo
    1. Bump the version number in the version.txt file for the plugin modified.
    2. Update CHANGELOG.md
    3. Merge your PR

### Minor/Major releases
Minor and major releases need to change both this repository and the
[Helm chart repo](https://github.com/FairwindsOps/charts/).

The steps are:
1. Modify the [Helm chart](https://github.com/FairwindsOps/charts/stable/insights-agent)
    1. Clone the helm charts repo
        1. `git clone https://github.com/FairwindsOps/charts`
        2. `git checkout -b yourname/update-insights-agent`
    1. Bump the version number in:
        2. stable/insights-agent/Chart.yaml
        3. stable/insights-agent/values.yaml
    2. Make any necessary changes to the chart to support the new version of the plugin (e.g. new RBAC permissions)
    3. **Don't merge yet!**
2. Create a PR for this repo
    1. Create a new branch named `yourname/new-feature`
    2. Bump the version number in the version.txt file for the plugin modified.
    3. Merge your PR
3. Create and merge a PR for your changes to the Helm chart

