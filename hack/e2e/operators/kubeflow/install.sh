#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Kubeflow training-operator (github.com/kubeflow/training-operator; also serves
# kubeflow.org/v1 MPIJob).
# shellcheck disable=SC2154  # KUBEFLOW_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> Kubeflow training-operator ${KUBEFLOW_VERSION}"
  # --force-conflicts: a reused cluster can hit field-ownership conflicts on rerun
  # (matches the other operators' applies).
  kubectl apply --server-side --force-conflicts -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=${KUBEFLOW_VERSION}"
  rollout_wait kubeflow deploy/training-operator
}

main "$@"
