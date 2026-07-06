#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for k8s-nim-operator: a throwaway NIMService (fictive CPU image) must
# reach state=Ready. Run after install.sh; uses run_smoke from _common.sh.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: nimservice/nim-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "nimservice/nim-smoke" "jsonpath={.status.state}=Ready" "300s" default
