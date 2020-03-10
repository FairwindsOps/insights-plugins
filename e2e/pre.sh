set -eo pipefail
mkdir output
for filename in deploy/*.config; do
    file="$(basename $filename)"
    docker-pull -f $filename
    docker-build -f $filename
    docker-save -f $filename -o output/${file%.*}.tgz
done

docker cp . e2e-command-runner:/workspace