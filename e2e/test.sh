set -eo pipefail
cd /workspace
for filename in deploy/*.config; do

    file="$(basename $filename)"
    baseFile="${file%.*}"
    export BASH_SOURCE=$filename
    . $filename
    if [ "$RUN_TEST" = "true" ]
    then
        cat ./e2e/base.yaml | sed -e "s/<TESTNAME>/$baseFile/g" | sed -e "s/<TESTIMAGE>/${DOCKERTAG//\//\\\/}:$CI_BRANCH/g" | kubectl apply -f -
        kubectl wait --for=condition=running job/$baseFile --timeout=120s
        kubectl cp job/$baseFile:/output output/$baseFile -c insights-sleep
        kubectl delete job $baseFile
        ls output/*
    fi
done