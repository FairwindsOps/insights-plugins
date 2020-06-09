# CI

A utility for the CI/CD integration of Fairwinds Insights.

Create a configuration file in the root of your project named `fairwinds-insights.yaml`, here's an example.

```yaml
images:
  folder: ./temp/images
  docker: # Saves images from Docker.
  - image1:tag
  - image2:tag
manifests:
  helm: # Runs a helm template for files to process
  - name: prod
    path: ./deploy/test
    variableFile: ./deploy/prod
    variables:
      x: y
      a: b
  yaml:
  - ./deploy/test.yaml # Processes any of the files present here.
options:
  tempFolder: ./temp/insights
  organization: example-co
  setExitCode: true  # return a non-zero exit code if the scores returned don't meet the thresholds.
  junitOutput: ./temp/insights.xml # Output action items as JUnit XML
  repositoryName: FairwindsOps/insights-plugins # Optional, defaults to Git Origin
  newActionItemThreshold: 5
  severityThreshold: danger # Could also be a number between 0 and 1. Fails if any new action item has a severity above this threshold.
```

If you're running Docker locally then you can execute the CI with `docker run -v $PWD:/insights  -e FAIRWINDS_TOKEN=<CI token from Insights> -it quay.io/fairwinds/insights-ci:<tag>` if you set the `junitOutput` setting then you'll need to `docker cp` the resulting file out of the container.