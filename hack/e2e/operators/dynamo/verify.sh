#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for dynamo-platform: a throwaway DynamoGraphDeployment must reach
# state=successful. run_smoke retries the apply past the DGD webhook warmup.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: dynamographdeployment/dynamo-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "dynamographdeployment/dynamo-smoke" "jsonpath={.status.state}=successful" "300s" default
