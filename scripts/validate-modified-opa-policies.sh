#! /bin/bash
set -eo pipefail

failed_policies=()
opa_base_dir='plugins/opa/examples-v2'

# List names of policies whos files have been modified since the main git branch.
for p in $(git diff --name-only --exit-code --no-renames origin/main ${opa_base_dir} |cut -d/ -f4 |sort -u) ; do
    echo "validating OPA policy $p"
    set +e
    insights-cli validate opa -b ${opa_base_dir}/${p}
    if [[ $? -gt 0 ]]; then
      failed_policies+=($p)
    fi
    set -e
    echo "finished validating OPA policy $p"
done

if [[ -n $BASH_ENV ]]; then
  echo "export FAILED_OPA_POLICIES_MARKDOWN=''" >> ${BASH_ENV}
fi

if (( ${#failed_policies[@]} != 0 )); then
    echo "The following OPA policies have failed validation:"
    for fp in "${failed_policies[@]}"; do
      if [[ -n $BASH_ENV ]]; then
        echo "FAILED_OPA_POLICIES_MARKDOWN+='- ${fp}\n'">> ${BASH_ENV}
      fi
      echo $fp
    done
    exit 1
else
    echo "All OPA policies have passed validation."
fi
