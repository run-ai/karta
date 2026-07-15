#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# kai-scheduler + Grove. Installed together: kai-scheduler is Grove's gang-scheduler
# backend. dynamo depends on this module (see deps_of in up.sh). Ships a values.yaml.
# shellcheck disable=SC2154  # KAI_SCHEDULER_VERSION/GROVE_VERSION come from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> kai-scheduler ${KAI_SCHEDULER_VERSION} + Grove ${GROVE_VERSION}"
  # Grove is Dynamo's multinode orchestrator; it publishes a multi-arch chart and
  # images (no source build needed). kai-scheduler is Grove's gang-scheduler backend.
  helm upgrade -i kai-scheduler oci://ghcr.io/kai-scheduler/kai-scheduler/kai-scheduler \
    --version "${KAI_SCHEDULER_VERSION}" -n kai-scheduler --create-namespace --wait --timeout 5m >/dev/null
  # Trim scheduler profiles to the backends actually installed (default-scheduler +
  # kai); the chart default also lists volcano/lpx, whose absent CRDs crash the
  # operator at startup. The chart installs the Grove CRDs itself.
  helm upgrade -i grove-operator oci://ghcr.io/ai-dynamo/grove/grove-charts \
    --version "${GROVE_VERSION}" -n grove-system --create-namespace \
    -f "${MODULE_DIR}/values.yaml" --wait --timeout 5m >/dev/null
  rollout_wait grove-system deploy/grove-operator
  # Health probes are enabled in values.yaml, so the rollout above only completes
  # once the webhook is serving - the smoke apply will not race it.
}

main "$@"
