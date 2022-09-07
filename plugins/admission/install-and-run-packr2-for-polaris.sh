#!/usr/bin/env sh
set -e
# Install packr2 and generate packr boxes for Polaris.
# This is required by the admission controller plugin.
#
# IDeally this script is called with $GOBIN already set to a temporary
# directory, where packr2 will be installed.
if [ "x${GOBIN}" != "x" ] ; then
  PATH=$GOBIN:$PATH
fi
go install github.com/gobuffalo/packr/v2/packr2@latest
if [ "x${GOMODCACHE}" == "x" ] ; then
  echo "The GOMODCACHE environment variable should be set to a temporary location for $0 to run packr2 against a cached version of fairwindsops/polaris, as downloaded via go mod download outside of this script."
  exit 1
fi

# On Mac OS, the go module cache isn't user-writable and packr2 fails confusingly.
chmod -R +w  $GOMODCACHE/github.com/fairwindsops/polaris*
cd $GOMODCACHE/github.com/fairwindsops/polaris*
packr2
