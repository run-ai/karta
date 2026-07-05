# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Shared helpers for the per-operator e2e modules under hack/e2e/operators/.
# Sourced by hack/e2e/up.sh, so every operator module can use these. Each module
# is a sourced fragment that defines operator_install() and, if it ships a
# smoke.yaml, sets SMOKE_TARGET / SMOKE_WAIT (optionally SMOKE_TIMEOUT /
# SMOKE_NS) at source time. Reference co-located files via "${MODULE_DIR}/...".
#
# All helpers are bash 3.2 safe (no associative arrays, no mapfile).
# shellcheck shell=bash

# rollout_wait <namespace> <deployment> [timeout]
# Wait for a Deployment to finish rolling out. Default timeout 180s.
rollout_wait() {
  local ns="$1" deploy="$2" timeout="${3:-180s}"
  kubectl -n "${ns}" rollout status "deploy/${deploy}" --timeout="${timeout}"
}

# apply_with_retry <file-or-url> [tries] [sleep_secs] [extra kubectl apply args...]
# kubectl apply, retried while a validating webhook is still warming up. The apply
# runs inside an if so set -e does not abort on an expected transient failure.
apply_with_retry() {
  local target="$1"; shift
  local tries="${1:-5}"; [ "$#" -gt 0 ] && shift || true
  local sleep_secs="${1:-10}"; [ "$#" -gt 0 ] && shift || true
  local i
  for i in $(seq 1 "${tries}"); do
    if kubectl apply "$@" -f "${target}"; then
      return 0
    fi
    echo "    apply failed (webhook warming up?); retry ${i}/${tries}" >&2
    sleep "${sleep_secs}"
  done
  return 1
}

# run_smoke <manifest> <target> <wait-expr> [timeout] [namespace]
# Uniform smoke test: apply the throwaway resource (retried past webhook warmup),
# wait for the stable state, then delete. <wait-expr> is passed to kubectl wait
# --for=, so it handles both "condition=Ready" and "jsonpath={.status.x}=y".
run_smoke() {
  local manifest="$1" target="$2" wait_expr="$3" timeout="${4:-300s}" ns="${5:-default}"
  apply_with_retry "${manifest}" 6 10
  # Capture the wait result so the throwaway resource is always cleaned up, and so
  # a failed smoke propagates regardless of how run_smoke is called (a bare call
  # under set -e, or in an if condition where set -e would not fire).
  local rc=0
  kubectl wait --for="${wait_expr}" "${target}" -n "${ns}" --timeout="${timeout}" || rc=$?
  kubectl delete -f "${manifest}" --wait=false || true
  return "${rc}"
}

# preload_image <source-ref> <local-tag>
# Pull an upstream image and load it into kind under a local tag, so the workload
# does not pull it mid-test.
preload_image() {
  local src="$1" tag="$2"
  docker pull "${src}"
  docker tag "${src}" "${tag}"
  kind load docker-image "${tag}" --name "${CLUSTER_NAME}"
}

# build_and_load_image <context-dir> <local-tag>
# Build a local image from a build context and load it into kind.
build_and_load_image() {
  local ctx="$1" tag="$2"
  docker build -t "${tag}" "${ctx}"
  kind load docker-image "${tag}" --name "${CLUSTER_NAME}"
}

# ensure_secret <namespace> <name> <literal k=v> [more k=v...]
# Idempotently create/update an opaque secret via apply.
ensure_secret() {
  local ns="$1" name="$2"; shift 2
  local args=() kv
  for kv in "$@"; do args+=(--from-literal="${kv}"); done
  kubectl create secret generic "${name}" -n "${ns}" "${args[@]}" \
    --dry-run=client -o yaml | kubectl apply -f -
}

# arch_ray_ref <ray-version>
# Echo the arch-native rayproject/ray image ref (the amd64 image under qemu
# emulation crash-loops when a RayJob runs real work).
arch_ray_ref() {
  case "$(uname -m)" in
    arm64 | aarch64) echo "rayproject/ray:${1}-aarch64" ;;
    *) echo "rayproject/ray:${1}" ;;
  esac
}
