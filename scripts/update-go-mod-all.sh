#! /bin/bash
set -eo pipefail

go clean -modcache
for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    cd $d
    name=$(echo $d | sed 's/\.\/plugins\///' | sed 's/\///')
    if test -f "go.mod"; then
        echo "updating..."
        mv go.mod old-go.mod
        rm -f go.mod
        rm -f go.sum
        go mod init github.com/fairwindsops/insights-plugins/$name
        if cat old-go.mod | grep replace; then
          cat old-go.mod | grep replace >> go.mod
        fi
        rm old-go.mod
        go get -d ./...
        go mod tidy
    fi
    cd ../..
done

