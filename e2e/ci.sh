set -eo pipefail
cd /workspace
. ./env.sh
for filename in deploy/*.config; do

    file="$(basename $filename)"
    baseFile="${file%.*}"
    
    . $filename
    cat base.yaml | sed "s/<TESTNAME>/$baseFile/g" | sed "s/<TESTIMAGE>/$DOCKERTAG:$CI_BRANCH/g" | kubetl apply -f -
    kubectl wait --for=condition=running job/$baseFile --timeout=120s
    kubectl cp job/$baseFile:/output output/$baseFile -c insights-sleep
    kubectl delete job $baseFile
    ls output/*
done