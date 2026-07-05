# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# KubeRay operator (github.com/ray-project/kuberay). Sourced by hack/e2e/up.sh;
# helpers and the module contract live in hack/e2e/operators/_common.sh. Ships a smoke test.
# shellcheck shell=bash
# shellcheck disable=SC2154  # KUBERAY_VERSION/RAY_VERSION provided by the orchestrator

SMOKE_TARGET="raycluster/ray-smoke"
SMOKE_WAIT="jsonpath={.status.state}=ready"
SMOKE_TIMEOUT="480s"

operator_install() {
  echo "==> KubeRay ${KUBERAY_VERSION}"
  helm repo add kuberay https://ray-project.github.io/kuberay-helm/ >/dev/null 2>&1 || true
  helm repo update kuberay >/dev/null
  helm upgrade -i kuberay-operator kuberay/kuberay-operator --version "${KUBERAY_VERSION}" \
    -n ray --create-namespace >/dev/null
  rollout_wait ray kuberay-operator 120s
  # Pre-load the Ray image into kind as ray-e2e:local so the RayCluster and RayJob
  # cases do not pull ~3GB from Docker Hub mid-test. arch_ray_ref picks the
  # arch-native variant (the amd64 image under qemu crash-loops running a RayJob).
  preload_image "$(arch_ray_ref "${RAY_VERSION}")" ray-e2e:local
}
