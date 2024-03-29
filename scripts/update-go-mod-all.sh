#! /bin/bash
set -eo pipefail

go version
go clean -modcache
failed=0
for d in ./plugins/*/ ; do
    echo -e "\n\n\n\n$d"
    if [[ $SKIP == *"$d"* ]]; then
      echo "skipping!"
      continue
    fi
    cd $d
    name=$(echo $d | sed 's/\.\/plugins\///' | sed 's/\///')
    if test -f "go.mod"; then
        echo "updating..."
        if [[ -z $UPDATE_PKG ]]; then
          go get -u -d ./...
        elif [[ $UPDATE_PKG != "none" ]]; then
          go get -u -d $UPDATE_PKG
        fi
        echo -e "\ntidying"
        go mod tidy
        echo -e "\ntesting"
        if ! go test ./...; then
          echo "TESTS FAILED FOR $name"
          failed=1
        fi
    fi
    cd ../..
done

if [[ $failed -eq 1 ]]; then
  echo "Some tests failed. See above"
  exit 1
fi
