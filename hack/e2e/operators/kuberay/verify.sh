#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for KubeRay: a throwaway RayCluster must reach state=ready.
# shellcheck disable=SC2154  # RAY_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: raycluster/ray-smoke"
# Render RAY_VERSION (global.env) into rayVersion so it matches the preloaded image.
rendered="$(mktemp)"
trap 'rm -f "${rendered}"' EXIT
sed "s|\${RAY_VERSION}|${RAY_VERSION}|g" "${MODULE_DIR}/smoke.yaml" >"${rendered}"
run_smoke "${rendered}" "raycluster/ray-smoke" "jsonpath={.status.state}=ready" "480s" default
