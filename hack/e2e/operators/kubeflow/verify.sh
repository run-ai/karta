#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Smoke tests: a throwaway PyTorchJob (training-operator) must reach Running and a
# throwaway v2beta1 MPIJob (mpi-operator) must reach Succeeded.
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

echo "==> smoke: pytorchjob/pytorch-smoke"
run_smoke "${MODULE_DIR}/smoke.yaml" "pytorchjob/pytorch-smoke" "condition=Running" "240s" default

echo "==> smoke: mpijob/mpi-smoke"
run_smoke "${MODULE_DIR}/mpi-smoke.yaml" "mpijob/mpi-smoke" "condition=Succeeded" "240s" default
