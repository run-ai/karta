#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# dynamo-platform (real operator + mocker DGD). Standalone: run via up.sh or
# directly (bash install.sh). Depends on grove (see deps_of in up.sh). Sources the
# shared helpers, which also load global.env.
# shellcheck disable=SC2154  # DYNAMO_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> dynamo-platform ${DYNAMO_VERSION} (real operator + mocker DGD)"
  # The platform chart is published on NGC as an https .tgz (anonymous). etcd is
  # off by default; the mocker workers need it for the distributed runtime. The
  # operator and dynamo-planner images are public and multi-arch. Fetch into a temp
  # dir cleaned up on exit rather than leaving an artifact in /tmp.
  local tmp; tmp="$(mktemp -d)"
  trap 'rm -rf "${tmp}"' EXIT
  curl -fsSL -o "${tmp}/dynamo-platform.tgz" \
    "https://helm.ngc.nvidia.com/nvidia/ai-dynamo/charts/dynamo-platform-${DYNAMO_VERSION}.tgz"
  helm upgrade -i dynamo-platform "${tmp}/dynamo-platform.tgz" -n dynamo-system --create-namespace \
    --set global.etcd.install=true --wait --timeout 8m >/dev/null
  # Dummy HF token: the mocker references the secret but downloads no model.
  ensure_secret default hf-token-secret HF_TOKEN=dummy
}

main "$@"
