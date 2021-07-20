#! /bin/bash
set -eo pipefail

go clean -modcache
for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    cd $d
    if test -f "go.mod"; then
      if cat go.mod | grep $1; then
        echo "updating..."
        go get -u $1
        go mod tidy
      fi
    fi
    cd ../..
done

