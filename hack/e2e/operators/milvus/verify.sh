#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for milvus-operator: a throwaway standalone Milvus must reach
# MilvusReady. Run after install.sh; uses run_smoke from _common.sh (apply, wait, delete).
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: milvus/milvus-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "milvus/milvus-smoke" "condition=MilvusReady" "480s" default
