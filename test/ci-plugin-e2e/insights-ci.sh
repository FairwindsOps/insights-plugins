#!/usr/bin/env bash
# shellcheck disable=SC1003
set -e

script_version=5.0
hostname=https://stable-main.k8s.insights.fairwinds.com
image_version=$CI_SHA1

# Based on https://gist.github.com/pkuczynski/8665367
# https://github.com/jasperes/bash-yaml MIT license

parse_yaml() {
    local yaml_file=$1
    local prefix=$2
    local s
    local w
    local fs

    s='[[:space:]]*'
    w='[a-zA-Z0-9_.-]*'
    fs="$(echo @|tr @ '\034')"

    (
        sed -e '/- [^\“]'"[^\']"'.*: /s|\([ ]*\)- \([[:space:]]*\)|\1-\'$'\n''  \1\2|g' |

        sed -ne '/^--/s|--||g; s|\"|\\\"|g; s/[[:space:]]*$//g;' \
            -e "/#.*[\"\']/!s| #.*||g; /^#/s|#.*||g;" \
            -e "s|^\($s\)\($w\)$s:$s\"\(.*\)\"$s\$|\1$fs\2$fs\3|p" \
            -e "s|^\($s\)\($w\)${s}[:-]$s\(.*\)$s\$|\1$fs\2$fs\3|p" |

        awk -F"$fs" '{
            indent = length($1)/2;
            if (length($2) == 0) { conj[indent]="+";} else {conj[indent]="";}
            vname[indent] = $2;
            for (i in vname) {if (i > indent) {delete vname[i]}}
                if (length($3) > 0) {
                    vn=""; for (i=0; i<indent; i++) {vn=(vn)(vname[i])("_")}
                    printf("%s%s%s%s=(\"%s\")\n", "'"$prefix"'",vn, $2, conj[indent-1],$3);
                }
            }' |

        sed -e 's/_=/+=/g' |

        awk 'BEGIN {
                FS="=";
                OFS="="
            }
            /(-|\.).*=/ {
                gsub("-|\\.", "_", $1)
            }
            { print }'
    ) < "$yaml_file"
}

create_variables() {
    local yaml_file="$1"
    local prefix="$2"
    eval "$(parse_yaml "$yaml_file" "$prefix")"
}

detect_and_set_ci_runner() {
  if [[ -n "${GITHUB_ACTIONS}" ]]; then
    ci_runner="github-actions"
  elif [[ -n "${CIRCLECI}" ]]; then
    ci_runner="circle-ci"
  elif [[ -n "${GITLAB_CI}" ]]; then
    ci_runner="gitlab"
  elif [[ -n "${TRAVIS}" ]]; then
    ci_runner="travis"
  elif [[ -n "${TF_BUILD}" ]]; then
    ci_runner="azure-devops"
  fi
}

print_ci_variables() {
  if [[ -n "${ci_runner}" ]]; then
    echo "CI runner ${ci_runner} detected"
  else
    echo "no CI runner detected"
  fi
}

detect_and_set_ci_runner
print_ci_variables

echo "Running version $image_version of CI script"
create_variables fairwinds-insights.yaml fairwinds_

fairwinds_images_folder=${fairwinds_images_folder:='./_insightsTempImages'}

mkdir -p $fairwinds_images_folder
for img in ${fairwinds_images_docker[@]}; do
    echo "Saving image $img"
    if [[ "$img" != "[]" && "$img" != "" ]]
    then
        if [[ "$(docker images -q "${img}" 2> /dev/null)" == "" ]]; then
            docker pull $img
        fi
        docker save $img -o $fairwinds_images_folder/$(basename $img | sed -e 's/[^a-zA-Z0-9]//g').tar
    fi
done

docker pull quay.io/fairwinds/insights-ci:$image_version

docker create --name insights-ci \
  -e SCRIPT_VERSION=$script_version \
  -e IMAGE_VERSION=$image_version \
  -e HOSTNAME=$hostname \
  -e CI_RUNNER=$ci_runner \
  -e FAIRWINDS_TOKEN \
  -e MASTER_HASH \
  -e CURRENT_HASH \
  -e COMMIT_MESSAGE \
  -e BRANCH_NAME \
  -e BASE_BRANCH \
  -e REPOSITORY_NAME \
  -e ORIGIN_URL \
  -e SKOPEO_ARGS \
  quay.io/fairwinds/insights-ci:$image_version

docker cp . insights-ci:/insights

# tries to interpolate variables, save new file content and override docker content and remove file
if command -v envsubst &> /dev/null
then
    echo "interpolating environment variables into fairwinds-insights.yaml"
    echo "$(envsubst < fairwinds-insights.yaml)" > fairwinds-insights-interpolated.yaml
    docker cp fairwinds-insights-interpolated.yaml insights-ci:/insights/fairwinds-insights.yaml
    rm fairwinds-insights-interpolated.yaml
else
    echo "envsubst command not found... interpolating environment variables into fairwinds-insights.yaml is not possible"
fi

failed=0
docker start -a insights-ci || failed=1

if [[ "$fairwinds_options_junitOutput" != "" ]]
then
    docker cp insights-ci:/insights/$fairwinds_options_junitOutput $fairwinds_options_junitOutput || echo "No jUnit output found"
fi
docker rm insights-ci
if [ "$failed" -eq "1" ]; then
    exit 1
fi
