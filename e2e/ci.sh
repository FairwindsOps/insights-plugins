set -eo pipefail
cd /workspace
for filename in deploy/*.config; do

    file="$(basename $filename)"
    baseFile="${file%.*}"
    kind load image-archive -f $filename -i output/$baseFile.tgz
    . k8s-read-config -f "$filename"
    cat base.yaml | sed "s/<TESTNAME>/$baseFile/g" | sed "s/<TESTIMAGE>/$DOCKERTAG/g" | kubetl apply -f -
    kubectl wait --for=condition=running job/$baseFile --timeout=120s
    kubectl cp job/$baseFile:/output output/$baseFile -c insights-sleep
    kubectl delete job $baseFile
    ls output/*
done