#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# KServe (real operator, Serverless on Knative + Kourier). Standalone: run via
# up.sh or directly (bash install.sh). Depends on knative (see deps_of in up.sh).
# Ships a config patch (disable-istio-vh.yaml). Sources the shared helpers, which
# also load global.env.
# shellcheck disable=SC2154  # KSERVE_VERSION/KUBE_RBAC_PROXY_VERSION come from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> KServe ${KSERVE_VERSION} (real operator, Serverless on Knative + Kourier)"
  # --force-conflicts: cert-manager-cainjector owns the webhook caBundle fields,
  # and on a reused cluster our own set-image/patch steps below already own the
  # rbac-proxy image and inferenceservice-config ingress. Reclaim them here; the
  # set-image and patch steps that follow re-assert those overrides.
  kubectl apply --server-side --force-conflicts -f "https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve.yaml"
  # Upstream pins gcr.io/kubebuilder/kube-rbac-proxy:${KSERVE_VERSION}, a tag that
  # registry no longer serves; the sidecar only guards metrics, so repoint it to a
  # maintained image, otherwise the pod never goes Ready and the webhook has no
  # endpoints.
  kubectl set image deployment/kserve-controller-manager -n kserve \
    kube-rbac-proxy="quay.io/brancz/kube-rbac-proxy:${KUBE_RBAC_PROXY_VERSION}"
  # Serverless KServe defaults to creating Istio VirtualServices; without Istio the
  # reconcile errors and PredictorReady/RoutesReady never go True. Route through
  # Knative/Kourier instead.
  kubectl patch cm inferenceservice-config -n kserve --type merge \
    --patch-file "${MODULE_DIR}/disable-istio-vh.yaml"
  rollout_wait kserve deploy/kserve-controller-manager 240s
  # ClusterServingRuntimes are validated by the webhook, so apply them only after
  # the controller pod is Ready; retry briefly in case the webhook is still warming.
  apply_with_retry "https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve-cluster-resources.yaml" \
    5 10 --server-side --force-conflicts
}

main "$@"
