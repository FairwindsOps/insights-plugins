#! /bin/bash
set -eo pipefail

go clean -modcache
for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    cd $d
    if test -f "go.mod"; then
        echo "updating..."
        rm -f go.mod
        rm -f go.sum
        go mod init
        go get -d ./...
        go mod tidy
    fi
    cd ../..
done

