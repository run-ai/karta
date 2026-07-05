#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Tear down the local Karta e2e cluster.
set -euo pipefail
DEFAULT_CLUSTER="karta-e2e"
CLUSTER_NAME="${CLUSTER_NAME:-${DEFAULT_CLUSTER}}"
# Mirror up.sh: a non-default cluster has its own kubeconfig; remove it too.
derived=""
if [ -z "${KUBECONFIG:-}" ] && [ "${CLUSTER_NAME}" != "${DEFAULT_CLUSTER}" ]; then
  KUBECONFIG="${HOME}/.kube/kind-${CLUSTER_NAME}.kubeconfig"
  export KUBECONFIG
  derived=1
fi
kind delete cluster --name "${CLUSTER_NAME}"
[ -n "${derived}" ] && rm -f "${KUBECONFIG}" || true
