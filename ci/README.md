# CI

A utility for the CI/CD integration of Fairwinds Insights.

Create a configuration file in the root of your project named `fairwinds-insights.yaml`, here's an example.

prod.yaml > <helm comment>

```yaml
images:
  folder: ./temp/images
  docker: # Saves images from Docker.
  - image1:tag
  - image2:tag
manifests:
  folder: ./temp/manifests
  helm: # Runs a helm template for files to process
  - name: prod
    path: ./deploy/test
    variables: ./deploy/prod
  yaml:
  - ./deploy/test.yaml # Processes any files here
options:
  tempFolder: ./temp/insights
  organization: example-co
  fail: true  # return a non-zero exit code if the scores returned don't meet the thresholds.
  junitOutput: ./temp/insights.xml # Output action items as JUnit XML
  scoreThreshold: 0.6
  scoreChangeThreshold: 0.4
```

If you're running Docker locally then you can execute the CI with `docker run -v $PWD:/insights  -e FAIRWINDS_TOKEN=<CI token from Insights> -it quay.io/fairwinds/insights-ci:<tag>` if you set the `junitOutput` setting then you'll need to `docker cp` the resulting file out of the container.