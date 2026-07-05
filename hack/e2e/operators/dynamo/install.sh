# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# dynamo-platform (real operator + mocker DGD). Sourced by hack/e2e/up.sh;
# helpers and the module contract live in hack/e2e/operators/_common.sh. Depends
# on grove (see deps_of in up.sh). Ships a smoke test (smoke.yaml); the smoke
# apply is retried past the DGD webhook warmup by run_smoke.
# shellcheck shell=bash
# shellcheck disable=SC2154  # DYNAMO_VERSION is provided by the orchestrator

SMOKE_TARGET="dynamographdeployment/dynamo-smoke"
SMOKE_WAIT="jsonpath={.status.state}=successful"
SMOKE_TIMEOUT="300s"

operator_install() {
  echo "==> dynamo-platform ${DYNAMO_VERSION} (real operator + mocker DGD)"
  # The platform chart is published on NGC as an https .tgz (anonymous). etcd is
  # off by default; the mocker workers need it for the distributed runtime. The
  # operator and dynamo-planner images are public and multi-arch.
  curl -fsSL -o /tmp/dynamo-platform.tgz \
    "https://helm.ngc.nvidia.com/nvidia/ai-dynamo/charts/dynamo-platform-${DYNAMO_VERSION}.tgz"
  helm upgrade -i dynamo-platform /tmp/dynamo-platform.tgz -n dynamo-system --create-namespace \
    --set global.etcd.install=true --wait --timeout 8m >/dev/null
  # Dummy HF token: the mocker references the secret but downloads no model.
  ensure_secret default hf-token-secret HF_TOKEN=dummy
}
