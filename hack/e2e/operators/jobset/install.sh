# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# JobSet operator (sigs.k8s.io/jobset). Sourced by hack/e2e/up.sh; helpers and
# the module contract live in hack/e2e/operators/_common.sh. Ships a smoke test.
# shellcheck shell=bash
# shellcheck disable=SC2154  # JOBSET_VERSION is provided by the orchestrator (versions.env)

SMOKE_TARGET="jobset/jobset-smoke"
SMOKE_WAIT="condition=Completed"
SMOKE_TIMEOUT="120s"

operator_install() {
  echo "==> JobSet ${JOBSET_VERSION}"
  kubectl apply --server-side -f "https://github.com/kubernetes-sigs/jobset/releases/download/${JOBSET_VERSION}/manifests.yaml"
  rollout_wait jobset-system jobset-controller-manager
}
