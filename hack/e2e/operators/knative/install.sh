#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Knative Serving + Kourier (real operator). Standalone: run via up.sh or directly
# (bash install.sh). Sources the shared helpers, which also load global.env.
# shellcheck disable=SC2154  # KNATIVE_VERSION/KOURIER_VERSION come from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> Knative Serving ${KNATIVE_VERSION} + Kourier ${KOURIER_VERSION} (real operator)"
  kubectl apply -f "https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-crds.yaml"
  kubectl apply -f "https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-core.yaml"
  for d in activator autoscaler controller webhook; do
    rollout_wait knative-serving "deploy/$d"
  done
  # Kourier is the networking layer; net-kourier tags lag serving patch releases.
  kubectl apply -f "https://github.com/knative/net-kourier/releases/download/${KOURIER_VERSION}/kourier.yaml"
  kubectl patch configmap/config-network -n knative-serving --type merge \
    -p '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
  kubectl patch configmap/config-domain -n knative-serving --type merge \
    -p '{"data":{"127.0.0.1.sslip.io":""}}'
  rollout_wait knative-serving deploy/net-kourier-controller
  rollout_wait kourier-system deploy/3scale-kourier-gateway
}

main "$@"
