#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# JobSet operator (sigs.k8s.io/jobset). Standalone: run via up.sh or directly
# (bash install.sh). Sources the shared helpers, which also load global.env.
# shellcheck disable=SC2154  # JOBSET_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> JobSet ${JOBSET_VERSION}"
  kubectl apply --server-side -f "https://github.com/kubernetes-sigs/jobset/releases/download/${JOBSET_VERSION}/manifests.yaml"
  rollout_wait jobset-system deploy/jobset-controller-manager
}

main "$@"
