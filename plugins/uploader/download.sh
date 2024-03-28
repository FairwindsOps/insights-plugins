#!/bin/sh
set -e

usage()
{
cat << EOF
usage: FAIRWINDS_TOKEN=secret $0 \
    --organzation acme-co \
    --cluster staging \
    --file foo.yaml \
    --datatype polaris \
   [--host https://insights.fairwinds.com]

This script uploads JSON to Fairwinds Insights.
EOF
}

organization=''
cluster=''
file=''
datatype=''
timeout='60'
host='https://insights.fairwinds.com'

while [ ! $# -eq 0 ]; do
    flag=${1##-}
    flag=${flag##-}
    value=${2}
    case "$flag" in
        o | organization)
            organization=${2}
            ;;
        c | cluster)
            cluster=${2}
            ;;
        f | file)
            file=${2}
            ;;
        d | datatype)
            datatype=${2}
            ;;
        h | host)
            host=${2}
            ;;
        *)
            usage
            exit
            ;;
    esac
    shift
    shift
done

if [[ -z $FAIRWINDS_TOKEN || -z $host || -z $organization || -z $cluster || -z $datatype || -z $file ]]; then
  usage
  exit 1
fi

url=$host/v0/organizations/$organization/clusters/$cluster/data/$datatype/config
status=0
results=$(curl -X GET $url \
  -L \
  -H "Authorization: Bearer ${FAIRWINDS_TOKEN//[$'\t\r\n']}" \
  -H "Accept: application/x-yaml" \
  -o $file \
  $CURL_EXTRA_ARGS \
  --fail 2>&1) || status=$?
if [ $status -ne 0 ]
then
    echo $results
    # If the response isn't a 404 this job will fail
    # 404s are ignored because it should mean that the config doesn't exist
    echo $results | grep 'The requested URL returned error: 404'
fi
