// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Icon } from '@iconify/react';
import { ApiProxy } from '@kinvolk/headlamp-plugin/lib';
import type {
  GraphNode,
  GraphSource,
} from '@kinvolk/headlamp-plugin/lib/components/resourceMap/graph/graphModel';
import { KubeObject } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { Box, Typography } from '@mui/material';
import { useEffect, useMemo, useState } from 'react';
import WorkloadTreePanel from './components/WorkloadTreePanel';
import type { GroupVersionKind } from './resources/karta';
import { KartaCR } from './resources/karta';

const kartaMapIcon = <Icon icon="mdi:file-tree" width="100%" height="100%" />;

interface WorkloadEntry {
  karta: KartaCR;
  workload: KubeObject;
}

// Listing across all namespaces uses the same collection path as a
// cluster-scoped list.
function listPath(gvk: GroupVersionKind, plural: string): string {
  const prefix = gvk.group ? `/apis/${gvk.group}/${gvk.version}` : `/api/${gvk.version}`;
  return `${prefix}/${plural}`;
}

function KartaNodeDetails({ node }: { node: GraphNode }) {
  const entry = node.data as WorkloadEntry;
  return (
    <Box sx={{ p: 1 }}>
      <Typography variant="h6" gutterBottom>
        {entry.workload.getName()}
      </Typography>
      <Typography variant="caption" color="text.secondary" component="div" gutterBottom>
        Karta workload tree ({entry.karta.getName()})
      </Typography>
      <WorkloadTreePanel karta={entry.karta} workload={entry.workload} variant="list" />
    </Box>
  );
}

// The set of workload kinds is data-driven (one per Karta CR), so workloads
// are fetched imperatively instead of with per-kind hooks, whose count must
// stay fixed across renders.
function useKartaWorkloads(): WorkloadEntry[] | null {
  const [kartas] = KartaCR.useList();
  const [entries, setEntries] = useState<WorkloadEntry[] | null>(null);

  useEffect(() => {
    if (!kartas) {
      return;
    }
    let cancelled = false;

    async function load() {
      const found: WorkloadEntry[] = [];
      const crdList = await ApiProxy.request(
        '/apis/apiextensions.k8s.io/v1/customresourcedefinitions'
      ).catch(() => null);
      const crds: any[] = crdList?.items ?? [];

      await Promise.all(
        (kartas ?? []).map(async karta => {
          const gvk = karta.rootGVK;
          if (!gvk) {
            return;
          }
          const crd = crds.find(
            item => item.spec?.group === gvk.group && item.spec?.names?.kind === gvk.kind
          );
          if (!crd) {
            return;
          }
          const list = await ApiProxy.request(listPath(gvk, crd.spec.names.plural)).catch(
            () => null
          );
          for (const item of list?.items ?? []) {
            found.push({ karta, workload: new KubeObject(item) });
          }
        })
      );

      if (!cancelled) {
        setEntries(found);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [kartas]);

  return entries;
}

export const kartaSource: GraphSource = {
  id: 'karta',
  label: 'Karta',
  icon: kartaMapIcon,
  useData() {
    const entries = useKartaWorkloads();

    return useMemo(() => {
      if (!entries) {
        return null;
      }
      const nodes: GraphNode[] = entries.map(entry => ({
        id: entry.workload.metadata.uid,
        kubeObject: entry.workload,
        subtitle: `Karta: ${entry.karta.getName()}`,
        detailsComponent: KartaNodeDetails,
        data: entry,
      }));
      return { nodes, edges: [] };
    }, [entries]);
  },
};
