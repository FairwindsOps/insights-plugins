#! /bin/bash
set -eo pipefail

if [[ ! " ${CHANGED[*]} " =~ " ci " ]]; then
    echo "The CI plugin was not changed. Exiting"
    exit 0
fi

DEMO_SERVER="https://stable-main.k8s.insights.fairwinds.com"
curl $DEMO_SERVER/v0/ping

THISDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
BASEDIR="$THISDIR/../.."
TESTDIR="$BASEDIR/test/ci-plugin-e2e"
CONFIG_FILE="$TESTDIR/fairwinds-insights.yaml"
ci_script="$TESTDIR/insights-ci.sh"

# Setup the $TESTDIR directory as a git repository
cd $TESTDIR
git config --global user.name "Insights CI"
git config --global user.email insights@fairwinds.com
git init
git add .
git commit -m "initial commit" || echo "Nothing new to commit"
git remote add origin https://github.com/acme-co/cicd-test || echo "Remote origin already added"

# Replace the required variables in the fairwinds-insights.yaml file
repoName=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 16)
sed -i "s/^  repositoryName: .*$/  repositoryName: ${repoName}/" $CONFIG_FILE

# Run the insights-ci script and check the Action Items
echo "FAIRWINDS_TOKEN=thisisacitoken" >> $BASH_ENV # FIXME: Not sure why we need to override the token this way...
echo "Running CI/CD on sample repo"
echo "The fairwinds-insights.yaml contents:"
cat $CONFIG_FILE

image_version=5.7 $ci_script &> output.txt || failed=false
if [[ -n $failed ]]; then
  cat output.txt
  echo "CI script returned non-zero. Exiting."
  exit 1
fi

echo -e "\n\nCI/CD is done. Validating output..."
cat output.txt
set +x

if ! grep -q "33 new Action Items" output.txt; then
  echo "[ERROR] Expected '33 new Action Items' not found in output:"
  exit 1
fi

if ! grep -q "0 fixed Action Items" output.txt; then
  echo "[ERROR] Expected '0 fixed Action Items' not found in output:"
  exit 1
fi
