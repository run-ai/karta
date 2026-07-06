#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for JobSet: a throwaway JobSet must reach Completed.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: jobset/jobset-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "jobset/jobset-smoke" "condition=Completed" "120s" default
