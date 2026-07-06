#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for LeaderWorkerSet: a throwaway LWS must reach Available. Run after
# install.sh; uses run_smoke from _common.sh (apply, wait, delete).
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: leaderworkerset/lws-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "leaderworkerset/lws-smoke" "condition=Available" "120s" default
