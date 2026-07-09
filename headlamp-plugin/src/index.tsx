// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Icon } from '@iconify/react';
import {
  K8s,
  registerKindIcon,
  registerMapSource,
  registerRoute,
  registerSidebarEntry,
  registerSidebarEntryFilter,
  Utils,
} from '@kinvolk/headlamp-plugin/lib';
import KartaList from './components/KartaList';
import WorkloadList from './components/WorkloadList';
import WorkloadTreeView from './components/WorkloadTreeView';
import { kartaSource } from './mapSource';
import { kartaListRoute, workloadListRoute, workloadTreeRoute } from './routes';

// Track whether the Karta CRD exists per cluster to hide the sidebar when the
// operator is not installed (same pattern as the Flux plugin).
const kartaInstalledByCluster: Record<string, boolean> = {};
const lastCheckedAt: Record<string, number> = {};
const inFlight: Record<string, boolean> = {};
const CHECK_TTL_MS = 30 * 1000;

function checkKartaInstalled(cluster: string) {
  const now = Date.now();
  const fresh = now - (lastCheckedAt[cluster] ?? 0) < CHECK_TTL_MS;
  if (inFlight[cluster] || fresh) {
    return;
  }
  inFlight[cluster] = true;

  const listFn = K8s.ResourceClasses.CustomResourceDefinition.apiList(
    crds => {
      kartaInstalledByCluster[cluster] = crds.some(
        crd => crd.jsonData?.metadata?.name === 'kartas.run.ai'
      );
      lastCheckedAt[cluster] = Date.now();
      inFlight[cluster] = false;
    },
    () => {
      kartaInstalledByCluster[cluster] = false;
      lastCheckedAt[cluster] = Date.now();
      inFlight[cluster] = false;
    },
    { cluster }
  );
  listFn();
}

registerSidebarEntryFilter(entry => {
  if (entry.name !== 'karta' && entry.parent !== 'karta') {
    return entry;
  }

  const cluster = Utils.getCluster() ?? '';
  checkKartaInstalled(cluster);

  if (kartaInstalledByCluster[cluster] === false) {
    return null;
  }
  return entry;
});

registerSidebarEntry({
  parent: null,
  name: 'karta',
  label: 'Karta',
  icon: 'mdi:file-tree',
  url: workloadListRoute.path,
});

registerSidebarEntry({
  parent: 'karta',
  name: 'karta-workloads',
  label: 'Workloads',
  url: workloadListRoute.path,
});

registerSidebarEntry({
  parent: 'karta',
  name: 'karta-definitions',
  label: 'Definitions',
  url: kartaListRoute.path,
});

registerRoute({
  path: workloadListRoute.path,
  sidebar: 'karta-workloads',
  name: workloadListRoute.name,
  exact: true,
  component: () => <WorkloadList />,
});

registerRoute({
  path: kartaListRoute.path,
  sidebar: 'karta-definitions',
  name: kartaListRoute.name,
  exact: true,
  component: () => <KartaList />,
});

registerRoute({
  path: workloadTreeRoute.path,
  sidebar: 'karta-workloads',
  name: workloadTreeRoute.name,
  exact: true,
  component: () => <WorkloadTreeView />,
});

registerKindIcon('Karta', {
  icon: <Icon icon="mdi:file-tree" width="70%" height="70%" />,
});

registerMapSource(kartaSource);
