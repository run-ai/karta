#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for KServe: a throwaway InferenceService must reach Ready.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: isvc/kserve-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "isvc/kserve-smoke" "condition=Ready" "300s" default
