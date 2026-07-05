# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# milvus-operator (real standalone Milvus). Sourced by hack/e2e/up.sh; helpers
# and the module contract live in hack/e2e/operators/_common.sh. Ships a smoke
# test (smoke.yaml): a standalone Milvus must reach MilvusReady.
# shellcheck shell=bash
# shellcheck disable=SC2154  # MILVUS_OPERATOR_VERSION is provided by the orchestrator

SMOKE_TARGET="milvus/milvus-smoke"
SMOKE_WAIT="condition=MilvusReady"
SMOKE_TIMEOUT="480s"

operator_install() {
  echo "==> milvus-operator ${MILVUS_OPERATOR_VERSION} (real standalone Milvus)"
  helm repo add milvus-operator https://zilliztech.github.io/milvus-operator/ >/dev/null 2>&1 || true
  helm repo update milvus-operator >/dev/null
  helm upgrade -i milvus-operator milvus-operator/milvus-operator \
    --version "${MILVUS_OPERATOR_VERSION}" -n milvus-operator --create-namespace --wait --timeout 5m >/dev/null
}
