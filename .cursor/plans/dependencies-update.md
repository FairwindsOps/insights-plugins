# Go Workspace Dependency Update Plan

Objective:
Safely update dependencies across all Go modules in this workspace.

Rules:
- Do NOT refactor unrelated code
- Do NOT change Go versions unless required
- Prefer minimal diffs
- Stop and report if tests fail

Steps:

1. Discover modules
   - List all go.mod files in the workspace
   - Confirm they are part of go.work

2. Sync workspace
   - Run: go work sync

3. Update dependencies (per module)
   For each module:
   - Run: go get -u ./...
   - Run: go mod tidy

4. Update docker dependencies - some modules relies on binaries being installed in their docker images, update then as well, here are the modules and dependency list

For `kubectlVersion` use the one with `kubernetes-` prefix.
Version number below are just for demonstration, not the latest used.

- ci:
   - trivyVersion=0.68.1 (source: https://github.com/aquasecurity/trivy/tags)
   - polarisVersion=10.1.3 (source: https://github.com/FairwindsOps/polaris/tags)
   - plutoVersion=5.22.7 (source: https://github.com/FairwindsOps/pluto/tags)
   - helmVersion=4.0.2 (source: https://github.com/helm/helm/tags)

- cloud-costs:
   - CLOUD_SDK_VERSION=526.0.1 (source: https://docs.cloud.google.com/sdk/gcloud)

- kube-bench:
   - kubectlVersion=1.34.3 (source https://github.com/kubernetes/kubectl/tags)

- kubectl:
   - kubectlVersion=1.34.3 (source https://github.com/kubernetes/kubectl/tags)

- kyverno-policy-sync:
   - kubectlVersion=1.34.3 (source https://github.com/kubernetes/kubectl/tags)

- trivy:
   - trivyVersion=0.68.1 (source: https://github.com/aquasecurity/trivy/tags)
   - kubectlVersion=1.34.3 (source https://github.com/kubernetes/kubectl/tags)
   - ENV CLOUD_SDK_VERSION=526.0.1 (source: https://docs.cloud.google.com/sdk/gcloud)

- downloader: 
   - kubectlVersion=1.34.3 (source https://github.com/kubernetes/kubectl/tags)

4. For each updated sub-module
  - Update `CHANGELOG.md`, for each updated lib append a bullet to the message `* Bump library X to version X.Y.Z`
  - Update `version.txt` by bumping a minor version

5. Review changes
   - Summarize updated dependencies
   - Highlight major version bumps
   - Flag potential breaking changes

6. Commit
   - Commit message format:
     chore(deps): Bump library dependencies (YYYY-MM-DD)

Output:
- Summary of updated dependencies
- Test results
- Any risk notes
