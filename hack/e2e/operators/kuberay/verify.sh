#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for KubeRay: a throwaway RayCluster must reach state=ready.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: raycluster/ray-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "raycluster/ray-smoke" "jsonpath={.status.state}=ready" "480s" default
