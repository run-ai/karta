// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Route names and paths shared by registrations and links. Cluster-scoped
// workloads use '-' as the namespace path segment.
export const workloadListRoute = {
  name: 'karta-workloads',
  path: '/karta/workloads',
};

export const kartaListRoute = {
  name: 'karta-definitions',
  path: '/karta/definitions',
};

export const workloadTreeRoute = {
  name: 'karta-workload-tree',
  path: '/karta/workloads/:karta/:namespace/:name',
};
