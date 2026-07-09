#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Kubeflow training-operator (PyTorchJob) + the standalone kubeflow mpi-operator for
# kubeflow.org/v2beta1 MPIJob (the newer MPI path, matching run:ai self-hosted).
# shellcheck disable=SC2154  # KUBEFLOW_VERSION/MPI_OPERATOR_VERSION from global.env via _common.sh
set -euo pipefail
MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${MODULE_DIR}/../_common.sh"

main() {
  echo "==> Kubeflow training-operator ${KUBEFLOW_VERSION}"
  # Start from a clean MPIJob CRD so neither apply below hits a stored-version
  # conflict on a reused cluster (we end on the mpi-operator's v2beta1 anyway).
  kubectl delete crd mpijobs.kubeflow.org --ignore-not-found
  # --force-conflicts: a reused cluster can hit field-ownership conflicts on rerun
  # (matches the other operators' applies).
  kubectl apply --server-side --force-conflicts -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=${KUBEFLOW_VERSION}"
  # Limit the training-operator to the job types we use and leave MPIJob to the
  # standalone mpi-operator below (matches run:ai self-hosted's --enable-scheme).
  # Without this it keeps trying to watch the v1 MPIJob CRD we drop and log-spams.
  kubectl patch deployment training-operator -n kubeflow \
    -p '{"spec":{"template":{"spec":{"containers":[{"name":"training-operator","args":["--enable-scheme=tfjob","--enable-scheme=pytorchjob","--enable-scheme=xgboostjob","--enable-scheme=jaxjob"]}]}}}}'
  rollout_wait kubeflow deploy/training-operator
  # Swap MPIJob from the training-operator's bundled v1 to the standalone
  # mpi-operator's v2beta1 (matches run:ai self-hosted). Drop the bundled v1 CRD
  # first - kube refuses to replace it in place while v1 is a stored version.
  echo "==> mpi-operator ${MPI_OPERATOR_VERSION} (kubeflow.org/v2beta1 MPIJob)"
  kubectl delete crd mpijobs.kubeflow.org --ignore-not-found
  apply_with_retry "https://raw.githubusercontent.com/kubeflow/mpi-operator/${MPI_OPERATOR_VERSION}/deploy/v2beta1/mpi-operator.yaml" \
    5 10 --server-side --force-conflicts
  rollout_wait mpi-operator deploy/mpi-operator
}

main "$@"
