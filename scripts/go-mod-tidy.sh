#! /bin/bash
set -eo pipefail

for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    cd $d
    name=$(echo $d | sed 's/\.\/plugins\///' | sed 's/\///')
    if test -f "go.mod"; then
        echo "tidying $name"
        go mod tidy
    fi
    cd ../..
done

