#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# milvus-operator (standalone Milvus).
# shellcheck disable=SC2154  # MILVUS_OPERATOR_VERSION comes from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> milvus-operator ${MILVUS_OPERATOR_VERSION}"
  helm repo add milvus-operator https://zilliztech.github.io/milvus-operator/ >/dev/null 2>&1 || true
  helm repo update milvus-operator >/dev/null
  helm upgrade -i milvus-operator milvus-operator/milvus-operator \
    --version "${MILVUS_OPERATOR_VERSION}" -n milvus-operator --create-namespace --wait --timeout 5m >/dev/null
}

main "$@"
