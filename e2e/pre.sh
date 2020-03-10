set -eo pipefail
mkdir output

docker cp . e2e-command-runner:/workspace
for filename in deploy/*.config; do
    file="$(basename $filename)"
    docker-pull -f $filename
    docker-build -f $filename
    docker-save -f $filename -o output/${file%.*}.tgz
    kind load image-archive -f $filename -i output/$baseFile.tgz
done
