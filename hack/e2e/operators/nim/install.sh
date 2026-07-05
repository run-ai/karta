# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# k8s-nim-operator with a fictive CPU NIM image (real NIMService test, no GPU or
# NGC token needed). Sourced by hack/e2e/up.sh; helpers and the module contract
# live in hack/e2e/operators/_common.sh. Ships an image build context (image/)
# and a smoke test. The operator chart is not published to a Helm/OCI registry,
# so it is fetched at install time from the pinned upstream git tag
# (NIM_OPERATOR_VERSION) rather than vendored in-repo.
# shellcheck shell=bash
# shellcheck disable=SC2154  # CLUSTER_NAME / NIM_OPERATOR_VERSION are provided by the orchestrator

SMOKE_TARGET="nimservice/nim-smoke"
SMOKE_WAIT="jsonpath={.status.state}=Ready"
SMOKE_TIMEOUT="300s"

operator_install() {
  echo "==> fake NIM image + k8s-nim-operator (real NIMService test)"
  build_and_load_image "${MODULE_DIR}/image" nim-cpu:e2e
  # The dev chart ships incomplete RBAC (cannot list computedomains/ingress/hpa/lws ->
  # cache-sync crashloop), so grant the operator broad access on this throwaway cluster.
  kubectl create clusterrolebinding k8s-nim-operator-admin \
    --clusterrole=cluster-admin --serviceaccount=nim-operator:k8s-nim-operator \
    --dry-run=client -o yaml | kubectl apply -f -
  # Install only when absent: the chart's pre-upgrade CRD-migration hook Job
  # crashloops when re-run, so plain "helm upgrade -i" is not idempotent here.
  if helm status k8s-nim-operator -n nim-operator >/dev/null 2>&1; then
    echo "    k8s-nim-operator release already present; skipping reinstall"
  else
    # The operator chart is not published to a Helm/OCI registry, only in-repo,
    # so fetch it from the pinned upstream tag at install time instead of
    # vendoring it. The tag tarball extracts to k8s-nim-operator-<version>/.
    local src chart
    src="$(mktemp -d)"
    curl -fsSL "https://github.com/NVIDIA/k8s-nim-operator/archive/refs/tags/${NIM_OPERATOR_VERSION}.tar.gz" \
      | tar -xz -C "${src}"
    chart="$(ls -d "${src}"/k8s-nim-operator-*/deployments/helm/k8s-nim-operator)"
    helm install k8s-nim-operator "${chart}" -n nim-operator --create-namespace >/dev/null
    rm -rf "${src}"
  fi
  rollout_wait nim-operator k8s-nim-operator
  # Dummy NGC secret: the operator injects NGC_API_KEY from it; the fictive image ignores it.
  ensure_secret default ngc-secret NGC_API_KEY=dummy-not-a-real-token
}
