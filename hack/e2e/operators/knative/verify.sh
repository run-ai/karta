#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for Knative Serving: a throwaway Knative Service must reach Ready, so
# a broken Serving install fails fast at provision time.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: ksvc/knative-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "ksvc/knative-smoke" "condition=Ready" "180s" default
