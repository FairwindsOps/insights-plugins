#!/bin/sh
set -e
set -x

if [ -z "$DEBUG" ]
then
    set +x
fi

# Debug: verify kubectl is a valid binary (not a script or error page)
if [ -n "$DEBUG" ] || [ -n "$UPLOADER_DEBUG_KUBECTL" ]; then
  echo "uploader: DEBUG kubectl check: path=/usr/local/bin/kubectl"
  ls -la /usr/local/bin/kubectl 2>&1 || true
  file /usr/local/bin/kubectl 2>&1 || true
  ( head -c 4 /usr/local/bin/kubectl | od -A n -t x1 ) 2>&1 || true
  /usr/local/bin/kubectl version --client 2>&1 || echo "uploader: kubectl version failed (exit $?)"
fi

usage()
{
cat << EOF
usage: FAIRWINDS_TOKEN=secret $0 \
    --organzation acme-co \
    --cluster staging \
    --file foo.json \
    --datatype polaris \
    --version 0.0.0 \
   [--timeout 120] \
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
        t | timeout)
            timeout=${2}
            ;;
        h | host)
            host=${2}
            ;;
        v | version)
            version=${2}
            ;;
        *)
            usage
            exit
            ;;
    esac
    shift
    shift
done

if [[ -z $host || -z $organization || -z $cluster || -z $datatype || -z $file || -z $version ]]; then
  usage
  exit 1
fi

echo "uploader: waiting for file=$file POD_NAME=${POD_NAME:-<unset>} (timeout=${timeout}s)"
attempts=0
while [ $attempts -lt $timeout ]
do
  attempts=$(( $attempts + 1 ))
  # Every 10 attempts query Kubernetes.
  # This avoids overloading the API servers.
  if [ -n "$DEBUG" ] || [ $(( $attempts % 10 )) -eq 0 ]; then
    echo "uploader: attempt $attempts/$timeout file_exists=$([ -f "$file" ] && echo yes || echo no)"
  fi

  if [ $(( $attempts % 10 )) -eq 0 ]; then
    # Check if any container inside this pod failed.
    if [ -n "$DEBUG" ] || [ -n "$UPLOADER_DEBUG_KUBECTL" ]; then
      echo "uploader: DEBUG attempt $attempts calling kubectl get pod POD_NAME=$POD_NAME"
    fi
    if kubectl get pod "$POD_NAME" -o go-template="{{range .status.containerStatuses}}{{.state.terminated.reason}}{{end}}" | grep Error; then
        url=$host/v0/organizations/$organization/clusters/$cluster/data/$datatype/failure

        # Get logs for container that's not insights-uploader and upload
        # data-binary to preserve newline characters.
        if [ "$SEND_FAILURES" = "true" ]; then
            set +x
            if [ -n "$DEBUG" ] || [ -n "$UPLOADER_DEBUG_KUBECTL" ]; then
              other_container=$(kubectl get pod "$POD_NAME" -o jsonpath="{.spec.containers[?(@.name != 'insights-uploader')].name}")
              echo "uploader: DEBUG fetching logs for POD_NAME=$POD_NAME container=$other_container"
            fi
            kubectl logs "$POD_NAME" -c $(kubectl get pod "$POD_NAME" -o jsonpath="{.spec.containers[?(@.name != 'insights-uploader')].name}") | \
            curl -X POST $url \
                -L \
                --data-binary @- \
                -H "Authorization: Bearer ${FAIRWINDS_TOKEN//[$'\t\r\n']}" \
                -H "Content-Type: application/json" \
                -H "X-Fairwinds-Agent-Version: `cat version.txt`" \
                -H "X-Fairwinds-Report-Version: ${version}" \
                -H "X-Fairwinds-Agent-Chart-Version: $FAIRWINDS_AGENT_CHART_VERSION" \
                $CURL_EXTRA_ARGS \
                --fail
            if [ -n "$DEBUG" ]
            then
                set -x
            fi
        fi
        exit 1
    fi
  fi
  if [ -f "$file" ]; then
    echo "uploader: found file=$file ($(wc -c < "$file" 2>/dev/null || echo 0) bytes), uploading..."
    url=$host/v0/organizations/$organization/clusters/$cluster/data/$datatype
    set +x
    curl -X POST "$url" \
      -L \
      --data-binary @"$file" \
      -H "Authorization: Bearer ${FAIRWINDS_TOKEN//[$'\t\r\n']}" \
      -H "Content-Type: application/json" \
      -H "X-Fairwinds-Agent-Version: `cat version.txt`" \
      -H "X-Fairwinds-Report-Version: ${version}" \
      -H "X-Fairwinds-Agent-Chart-Version: $FAIRWINDS_AGENT_CHART_VERSION" \
      $CURL_EXTRA_ARGS \
      --fail
    set -x
    exit 0
  fi
  sleep 1
done

echo "Timed out after $attempts seconds while waiting for file $file"
echo "If this keeps happening then you can consider increasing the timeout in your helm install with"
echo "--set $datatype.timeout=$((timeout * 10))"
exit 1
