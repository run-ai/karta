# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# kai-scheduler + Grove (real PodCliqueSet). Installed together: kai-scheduler is
# Grove's gang-scheduler backend. Sourced by hack/e2e/up.sh; helpers and the
# module contract live in hack/e2e/operators/_common.sh. dynamo depends on this
# module (see deps_of in up.sh). Ships a values.yaml and a smoke test.
# shellcheck shell=bash
# shellcheck disable=SC2154  # KAI_SCHEDULER_VERSION/GROVE_VERSION provided by the orchestrator

SMOKE_TARGET="podcliqueset/grove-smoke"
SMOKE_WAIT="jsonpath={.status.availableReplicas}=1"
SMOKE_TIMEOUT="180s"

operator_install() {
  echo "==> kai-scheduler ${KAI_SCHEDULER_VERSION} + Grove ${GROVE_VERSION} (real PodCliqueSet)"
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
  rollout_wait grove-system grove-operator
  # Health probes are enabled in values.yaml, so the rollout above only completes
  # once the webhook is serving - the smoke apply will not race it.
}
