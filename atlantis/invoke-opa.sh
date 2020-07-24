#!/bin/bash

set -e -o pipefail

echo "Sending plan to OPA at ${OPA_URL}"

plan_json=$(terraform${ATLANTIS_TERRAFORM_VERSION} show -json ${PLANFILE})

if [[ ! -z ${PULL_NUM} ]]; then
    pr="{\"owner\": \"${BASE_REPO_OWNER}\", \"name\": \"${BASE_REPO_NAME}\", \"pull_num\": ${PULL_NUM}}"
else
    pr="null"
fi

data="{\"pull_request\": ${pr}, \"plan\": ${plan_json}}"
curl -sS -X POST -d "${data}" -H "Content-type: application/json" ${OPA_URL}
