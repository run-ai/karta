#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for dynamo-platform: a throwaway DynamoGraphDeployment must reach
# state=successful. run_smoke retries the apply past the DGD webhook warmup.
# shellcheck disable=SC2154  # DYNAMO_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: dynamographdeployment/dynamo-smoke"
# Render DYNAMO_VERSION (global.env) into the smoke image tag so it tracks the
# installed platform version.
rendered="$(mktemp)"
trap 'rm -f "${rendered}"' EXIT
sed "s|\${DYNAMO_VERSION}|${DYNAMO_VERSION}|g" "${MODULE_DIR}/smoke.yaml" >"${rendered}"
run_smoke "${rendered}" "dynamographdeployment/dynamo-smoke" "jsonpath={.status.state}=successful" "300s" default
