#! /bin/bash
set -e
trap 'echo "Error on Line: $LINENO"' ERR
echo "Starting kubesec"
tmp_dir=/output/tmp
mkdir -p $tmp_dir
results_file=$tmp_dir/results.json
list_file=$tmp_dir/list.json

echo -e '{\n  "namespaces": {\n' > $results_file

TYPES=( "deployments" "statefulsets" "daemonsets" )

echo "Retrieving namespaces"
namespaces=$(kubectl get namespaces -o name)
IFS=$'\n' namespaces=($namespaces)
for ns_idx in "${!namespaces[@]}"; do
  namespace=${namespaces[$ns_idx]#namespace\/}
  echo -e "    \"$namespace\": {" >> $results_file

  for t in "${TYPES[@]}"
  do
    count=$(kubectl get $t -n $namespace -o name | wc -l)
    echo "found $count $t for namespace $namespace"
    kubectl get $t -n $namespace -o json | jq ".items[$ctrl_idx]" | awk "{print > \"$tmp_dir/obj\" NR \".json\";}"
    sync
    
      
    echo -e "      \"$t\": [" >> $results_file
    for ctrl_idx in $(seq 0 $((count-1)))
    do
      echo "scanning $t number $ctrl_idx"
      name=$(cat $list_file | jq ".items[$ctrl_idx].metadata.name")
      echo -e '        {' >> $results_file
      echo -e '          "name":' ${name}',' >> $results_file
      echo -e '          "namespace": "'${namespace}'",' >> $results_file
      echo -e '          "results": ' >> $results_file
      item_file="$tmp_dir/obj${ctrl_idx}.json"
      kubesec scan $item_file >> $results_file
      if [[ $ctrl_idx -lt $(( count - 1 )) ]]; then
        echo -e '        },' >> $results_file
      else
        echo -e '        }' >> $results_file
      fi
    done
    if [[ $t != "${TYPES[-1]}" ]]; then
      echo '      ],' >> $results_file
    else
      echo '      ]' >> $results_file
    fi
  done

  if [[ $ns_idx -lt $((${#namespaces[@]} - 1)) ]]; then
    echo -e '    },' >> $results_file
  else
    echo -e '    }' >> $results_file
  fi
done

echo -e '  }\n}' >> $results_file
mv $results_file /output/kubesec.json
