# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Kubeflow training-operator (github.com/kubeflow/training-operator; also serves
# kubeflow.org/v1 MPIJob). Sourced by hack/e2e/up.sh; helpers and the module
# contract live in hack/e2e/operators/_common.sh. Ships a smoke test.
# shellcheck shell=bash
# shellcheck disable=SC2154  # KUBEFLOW_VERSION is provided by the orchestrator

SMOKE_TARGET="pytorchjob/pytorch-smoke"
SMOKE_WAIT="condition=Running"
SMOKE_TIMEOUT="240s"

operator_install() {
  echo "==> Kubeflow training-operator ${KUBEFLOW_VERSION}"
  kubectl apply --server-side -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=${KUBEFLOW_VERSION}"
  rollout_wait kubeflow training-operator
}
