# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# LeaderWorkerSet operator (sigs.k8s.io/lws). Sourced by hack/e2e/up.sh; helpers
# and the module contract live in hack/e2e/operators/_common.sh. Ships a smoke test.
# shellcheck shell=bash
# shellcheck disable=SC2154  # LWS_VERSION is provided by the orchestrator (versions.env)

SMOKE_TARGET="leaderworkerset/lws-smoke"
SMOKE_WAIT="condition=Available"
SMOKE_TIMEOUT="120s"

operator_install() {
  echo "==> LeaderWorkerSet ${LWS_VERSION}"
  kubectl apply --server-side -f "https://github.com/kubernetes-sigs/lws/releases/download/${LWS_VERSION}/manifests.yaml"
  rollout_wait lws-system lws-controller-manager
}
