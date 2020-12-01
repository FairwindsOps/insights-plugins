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
- Be up to date and/or rebased on the main branch

## Creating a new release

* Make sure you update the `version.txt` in the plugin that you've changed
* Update the `CHANGELOG.md` in the plugin with any changes
* If you're changing the major/minor version, be sure to change the [Helm chart](https://github.com/FairwindsOps/charts/stable/insights-agent) accordingly.
