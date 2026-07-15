#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# KubeRay operator (github.com/ray-project/kuberay).
# shellcheck disable=SC2154  # KUBERAY_VERSION/RAY_VERSION come from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

# arch_ray_ref <ray-version>: the arch-native rayproject/ray image ref (the amd64
# image under qemu emulation crash-loops when a RayJob runs real work). Only
# kuberay needs the Ray image, so it lives here rather than in _common.sh.
arch_ray_ref() {
  case "$(uname -m)" in
    arm64 | aarch64) echo "rayproject/ray:${1}-aarch64" ;;
    *) echo "rayproject/ray:${1}" ;;
  esac
}

main() {
  echo "==> KubeRay ${KUBERAY_VERSION}"
  helm repo add kuberay https://ray-project.github.io/kuberay-helm/ >/dev/null 2>&1 || true
  helm repo update kuberay >/dev/null
  helm upgrade -i kuberay-operator kuberay/kuberay-operator --version "${KUBERAY_VERSION}" \
    -n ray --create-namespace >/dev/null
  rollout_wait ray deploy/kuberay-operator 120s
  # Pre-load the Ray image into kind as ray-e2e:local so the RayCluster and RayJob
  # cases do not pull ~3GB from Docker Hub mid-test. arch_ray_ref picks the
  # arch-native variant (the amd64 image under qemu crash-loops running a RayJob).
  preload_image "$(arch_ray_ref "${RAY_VERSION}")" ray-e2e:local
}

main "$@"
