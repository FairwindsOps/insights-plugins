#! /bin/bash
set -eo pipefail

go clean -modcache
for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    if [[ $SKIP == *"$d"* ]]; then
      echo "skipping!"
    fi
    cd $d
    name=$(echo $d | sed 's/\.\/plugins\///' | sed 's/\///')
    if test -f "go.mod"; then
        echo "updating..."
        if [[ -z $UPDATE_PKG ]]; then
          go get -u -d ./...
        else
          go get -u -d $UPDATE_PKG
        fi
        go mod tidy
        go test ./...
    fi
    cd ../..
done
