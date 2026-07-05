#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Provision a local kind cluster for the Karta e2e suite: builds and deploys the
# Karta operator, then installs the dependencies the suite needs (cert-manager,
# the fake-gpu-operator) and the upstream workload operators it exercises. The
# e2e tests connect to this cluster; they do not create it (see test/e2e).
#
# Usage:
#   ./hack/e2e/up.sh                 # install everything (default)
#   ./hack/e2e/up.sh jobset kuberay  # install only the named workload operators
#   ./hack/e2e/up.sh --list jobset   # print the resolved install plan and exit
#   ./hack/e2e/up.sh --help
#
# The always-on base (kind cluster, cert-manager, fake-gpu-operator, the Karta
# operator) is installed regardless of selection. Selecting a subset keeps a
# focused run light: a single worker plus the control-plane cannot hold every
# operator and a real Milvus/Knative/KServe workload at once.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_CLUSTER="karta-e2e"
CLUSTER_NAME="${CLUSTER_NAME:-${DEFAULT_CLUSTER}}"
IMAGE="${IMAGE:-karta-operator:e2e}"
# Each non-default cluster gets its own kubeconfig, so several clusters (for
# example two CI shards installing different operators) can be provisioned in
# parallel without racing on the shared current-context. The default cluster
# keeps the standard kubeconfig so plain kubectl works after e2e-up. Set
# KUBECONFIG explicitly to override either way.
if [ -z "${KUBECONFIG:-}" ] && [ "${CLUSTER_NAME}" != "${DEFAULT_CLUSTER}" ]; then
  KUBECONFIG="${HOME}/.kube/kind-${CLUSTER_NAME}.kubeconfig"
  mkdir -p "$(dirname "${KUBECONFIG}")"
  export KUBECONFIG
fi
# Version pins live in one place; each stays overridable from the environment.
source "${REPO_ROOT}/hack/e2e/versions.env"
# Shared helpers for the per-operator modules under hack/e2e/operators/.
OPERATORS_DIR="${REPO_ROOT}/hack/e2e/operators"
source "${OPERATORS_DIR}/_common.sh"

# Workload operators selectable on the command line, in canonical install order:
# a dependency always appears before its dependents (knative before kserve,
# grove before dynamo).
ALL_WORKLOADS=(lws jobset kuberay kubeflow knative kserve milvus grove dynamo nim)

# deps_of <workload> prints the workload operators that must be installed first.
deps_of() {
  case "$1" in
    kserve) echo "knative" ;; # KServe Serverless routes through Knative + Kourier
    dynamo) echo "grove" ;;   # Grove is Dynamo's multinode orchestrator
  esac
}

usage() {
  cat >&2 <<EOF
Usage: $0 [--list] [workload...]
  No workload args installs everything. Named args install the base plus only
  those workload operators (and their dependencies).
  Workloads: ${ALL_WORKLOADS[*]}
  --list    print the resolved install plan and exit
EOF
}

# --- base (always installed) -------------------------------------------------

# require_tools fails fast with a clear message if a needed binary is missing,
# rather than erroring cryptically partway through provisioning.
require_tools() {
  local missing=()
  for t in docker kind kubectl helm curl; do
    command -v "$t" >/dev/null 2>&1 || missing+=("$t")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    echo "error: missing required tools: ${missing[*]}" >&2
    echo "install them and re-run; see test/e2e/README.md prerequisites." >&2
    exit 1
  fi
}

setup_cluster() {
  echo "==> build operator image (${IMAGE})"
  # Build output is left visible: a failing image build is the first thing to debug.
  docker build --build-arg "GO_VERSION=${GO_VERSION}" \
    -f "${REPO_ROOT}/operator/Dockerfile" -t "${IMAGE}" "${REPO_ROOT}"

  if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    echo "==> reusing existing kind cluster (${CLUSTER_NAME})"
    # Re-export the kubeconfig: the cluster can outlive its context (a reset or
    # overwritten ~/.kube/config), and "kind create" is skipped on this path.
    kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null
  else
    echo "==> create kind cluster (${CLUSTER_NAME}, ${KIND_NODE_IMAGE})"
    kind create cluster --name "${CLUSTER_NAME}" --image "${KIND_NODE_IMAGE}" \
      --config "${REPO_ROOT}/hack/e2e/kind-config.yaml"
  fi
  # Pin kubectl to this cluster so every step below targets it, not a stale context.
  kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
  echo "==> load operator image"
  kind load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"
  kubectl wait --for=condition=Ready nodes --all --timeout=120s
  # Untaint the control-plane so its capacity is schedulable: the single worker
  # cannot hold every operator plus a real Milvus/Knative/KServe workload at once.
  kubectl taint nodes "${CLUSTER_NAME}-control-plane" \
    node-role.kubernetes.io/control-plane:NoSchedule- 2>/dev/null || true
}

install_cert_manager() {
  echo "==> cert-manager ${CERT_MANAGER_VERSION}"
  kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
  for d in cert-manager cert-manager-webhook cert-manager-cainjector; do
    kubectl -n cert-manager rollout status "deploy/${d}" --timeout=180s
  done
}

install_fake_gpu() {
  echo "==> fake-gpu-operator ${FAKE_GPU_VERSION}"
  kubectl label node "${CLUSTER_NAME}-worker" run.ai/simulated-gpu-node-pool=default --overwrite
  helm upgrade -i fake-gpu-operator oci://ghcr.io/run-ai/fake-gpu-operator/fake-gpu-operator \
    -n gpu-operator --create-namespace --version "${FAKE_GPU_VERSION}" \
    --set computeDomainDraPlugin.enabled=true >/dev/null
}

install_karta() {
  echo "==> Karta operator"
  kubectl apply --server-side -f "${REPO_ROOT}/charts/karta/crds/"
  helm upgrade -i karta "${REPO_ROOT}/charts/karta" -n karta-system --create-namespace \
    --set image.repository="${IMAGE%%:*}" --set image.tag="${IMAGE##*:}" \
    --set resources.limits.memory=512Mi >/dev/null
  kubectl -n karta-system rollout status deploy/karta-operator --timeout=120s
}

# --- selectable workload operators -------------------------------------------

# --- dispatch ----------------------------------------------------------------

# run_module <name>: source the operator's module, run its install hook, then run
# its smoke test if the module ships one. A module defines operator_install() and,
# when it has a smoke.yaml, sets SMOKE_TARGET/SMOKE_WAIT (optionally
# SMOKE_TIMEOUT/SMOKE_NS) at source time. MODULE_DIR/SMOKE_* are locals here and
# reach the sourced hook via bash dynamic scope, so they reset on every call.
run_module() {
  local name="$1"
  local dir="${OPERATORS_DIR}/${name}"
  [ -f "${dir}/install.sh" ] || { echo "error: no operator module at ${dir}" >&2; exit 1; }
  local MODULE_DIR="${dir}"
  local SMOKE_TARGET="" SMOKE_WAIT="" SMOKE_TIMEOUT="300s" SMOKE_NS="default"
  unset -f operator_install 2>/dev/null || true
  # shellcheck source=/dev/null
  source "${dir}/install.sh"
  operator_install
  if [ -f "${dir}/smoke.yaml" ]; then
    [ -n "${SMOKE_TARGET}" ] && [ -n "${SMOKE_WAIT}" ] ||
      { echo "error: ${name}/smoke.yaml present but SMOKE_TARGET/SMOKE_WAIT unset" >&2; exit 1; }
    echo "    smoke-test: ${SMOKE_TARGET}"
    run_smoke "${dir}/smoke.yaml" "${SMOKE_TARGET}" "${SMOKE_WAIT}" "${SMOKE_TIMEOUT}" "${SMOKE_NS}"
  fi
}

main() {
  local plan_only=false
  local requested=()
  for arg in "$@"; do
    case "$arg" in
      -h | --help) usage; exit 0 ;;
      --list | --plan) plan_only=true ;;
      -*) echo "unknown flag: $arg" >&2; usage; exit 2 ;;
      *) requested+=("$arg") ;;
    esac
  done

  # Resolve the selection: no names means every workload.
  local selected=()
  if [ "${#requested[@]}" -eq 0 ]; then
    selected=("${ALL_WORKLOADS[@]}")
  else
    for w in "${requested[@]}"; do
      printf '%s\n' "${ALL_WORKLOADS[@]}" | grep -qxF "$w" ||
        { echo "unknown workload: $w" >&2; usage; exit 2; }
    done
    selected=("${requested[@]}")
    for w in "${requested[@]}"; do
      for d in $(deps_of "$w"); do selected+=("$d"); done
    done
  fi

  # Build the ordered plan by walking the canonical order and keeping selected ones.
  local plan=()
  for w in "${ALL_WORKLOADS[@]}"; do
    printf '%s\n' "${selected[@]}" | grep -qxF "$w" && plan+=("$w") || true
  done

  if [ "$plan_only" = true ]; then
    echo "base: kind cluster, cert-manager, fake-gpu-operator, karta"
    if [ "${#plan[@]}" -gt 0 ]; then echo "workloads: ${plan[*]}"; else echo "workloads: (none)"; fi
    exit 0
  fi

  require_tools
  setup_cluster
  install_cert_manager
  install_fake_gpu
  if [ "${#plan[@]}" -gt 0 ]; then
    for w in "${plan[@]}"; do run_module "$w"; done
  fi
  install_karta

  echo "==> environment ready (cluster: ${CLUSTER_NAME})."
  if [ "${CLUSTER_NAME}" != "${DEFAULT_CLUSTER}" ]; then
    echo "    this cluster has its own kubeconfig: ${KUBECONFIG}"
    echo "    run the suite against it: make test-e2e CLUSTER_NAME=${CLUSTER_NAME}"
    echo "    (or: export KUBECONFIG=${KUBECONFIG})"
  else
    echo "    Run: make test-e2e"
  fi
}

main "$@"
