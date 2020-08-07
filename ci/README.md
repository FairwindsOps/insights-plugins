# CI

A utility for the CI/CD integration of Fairwinds Insights.

Create a configuration file in the root of your project named `fairwinds-insights.yaml`.

Here's a minimal example:
> Be sure to replace `acme-co` with your Insights organization!
```yaml
options:
  setExitCode: true
  organization: acme-co

images:
  docker:
  - nginx:1.18-alpine

manifests:
  yaml:
  - ./deploy/test.yaml
  helm:
  - name: prod
    path: ./deploy/chart
    variableFile: ./deploy/prod
  - name: staging
    path: ./deploy/chart
    variables:
      x: y
      a: b
```

For a full list of options, as well as additional documentation, visit
[insights.docs.fairwinds.com](https://insights.docs.fairwinds.com/features/continuous-integration/)
