#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke test for Kubeflow training-operator: a throwaway PyTorchJob must reach Running.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: pytorchjob/pytorch-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "pytorchjob/pytorch-smoke" "condition=Running" "240s" default
