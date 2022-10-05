#! /bin/bash
set -eo pipefail

mkdir tmp_repos
cd tmp_repos
git clone https://github.com/FairwindsOps/charts
git clone https://github.com/FairwindsOps/insights-plugins
git clone https://github.com/FairwindsOps/polaris
git clone https://github.com/FairwindsOps/goldilocks
git clone https://github.com/FairwindsOps/pluto
git clone https://github.com/FairwindsOps/nova

cd polaris
polaris_v=$(git describe --tags --abbrev=0)
cd ..

cd goldilocks
goldilocks_v=$(git describe --tags --abbrev=0)
cd ..

cd nova
nova_v=$(git describe --tags --abbrev=0)
cd ..

cd pluto
pluto_v=$(git describe --tags --abbrev=0)
cd ..

echo "VERSIONS:"
echo "polaris: $polaris_v"
echo "goldilocks: $goldilocks_v"
echo "nova: $nova_v"
echo "pluto: $pluto_v"

cd ..
rm -rf tmp_repos
