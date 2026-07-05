# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Knative Serving + Kourier (real operator). Sourced by hack/e2e/up.sh; helpers
# and the module contract live in hack/e2e/operators/_common.sh. Ships a smoke
# test (smoke.yaml): a throwaway Knative Service must reach Ready, so a broken
# Serving install fails fast at provision time (Grove's smoke-test pattern).
# shellcheck shell=bash
# shellcheck disable=SC2154  # KNATIVE_VERSION/KOURIER_VERSION provided by the orchestrator

SMOKE_TARGET="ksvc/knative-smoke"
SMOKE_WAIT="condition=Ready"
SMOKE_TIMEOUT="180s"

operator_install() {
  echo "==> Knative Serving ${KNATIVE_VERSION} + Kourier ${KOURIER_VERSION} (real operator)"
  kubectl apply -f "https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-crds.yaml"
  kubectl apply -f "https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-core.yaml"
  for d in activator autoscaler controller webhook; do
    rollout_wait knative-serving "$d"
  done
  # Kourier is the networking layer; net-kourier tags lag serving patch releases.
  kubectl apply -f "https://github.com/knative/net-kourier/releases/download/${KOURIER_VERSION}/kourier.yaml"
  kubectl patch configmap/config-network -n knative-serving --type merge \
    -p '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
  kubectl patch configmap/config-domain -n knative-serving --type merge \
    -p '{"data":{"127.0.0.1.sslip.io":""}}'
  rollout_wait knative-serving net-kourier-controller
  rollout_wait kourier-system 3scale-kourier-gateway
}
