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

4. For each updated sub-module
  - Update `CHANGELOG.md` using the message `Bump library dependencies`
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
