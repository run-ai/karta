// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Icon } from '@iconify/react';
import { Loader, SimpleTable } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import type { KubeObject } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { Box, Link as MuiLink, Typography } from '@mui/material';
import { Link as RouterLink } from 'react-router-dom';
import type { SubResource } from '../api/resources';

interface Row {
  resource: SubResource;
  depth: number;
}

// hierarchyRows orders resources depth-first so every object appears below
// its parent, indented one level deeper.
export function hierarchyRows(resources: SubResource[], workloadUid: string): Row[] {
  const childrenByParent = new Map<string, SubResource[]>();
  for (const resource of resources) {
    const parent = resource.parentUid ?? workloadUid;
    const siblings = childrenByParent.get(parent) ?? [];
    siblings.push(resource);
    childrenByParent.set(parent, siblings);
  }

  const rows: Row[] = [];
  const visit = (parentUid: string, depth: number) => {
    for (const resource of childrenByParent.get(parentUid) ?? []) {
      rows.push({ resource, depth });
      visit(resource.item.metadata.uid, depth + 1);
    }
  };
  visit(workloadUid, 0);
  return rows;
}

function NameCell({ row }: { row: Row }) {
  const { resource, depth } = row;
  const name = resource.item.metadata.name;
  return (
    <Box sx={{ pl: depth * 3, display: 'flex', alignItems: 'center', gap: 0.5 }}>
      {depth > 0 && (
        <Icon icon="mdi:subdirectory-arrow-right" width={14} style={{ opacity: 0.6 }} />
      )}
      {resource.href ? (
        <MuiLink component={RouterLink} to={resource.href}>
          {name}
        </MuiLink>
      ) : (
        <>{name}</>
      )}
    </Box>
  );
}

function age(creationTimestamp?: string): string {
  if (!creationTimestamp) {
    return '';
  }
  const seconds = Math.max(0, (Date.now() - new Date(creationTimestamp).getTime()) / 1000);
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m`;
  }
  if (seconds < 24 * 3600) {
    return `${Math.floor(seconds / 3600)}h`;
  }
  return `${Math.floor(seconds / (24 * 3600))}d`;
}

// WorkloadSubResources lists the live Kubernetes objects owned by the
// workload as a hierarchy: each object sits below its parent, linking to its
// Headlamp resource page.
export default function WorkloadSubResources({
  resources,
  workload,
}: {
  resources: SubResource[] | null;
  workload: KubeObject;
}) {
  if (!resources) {
    return <Loader title="Loading workload resources" />;
  }
  if (!resources.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No live resources owned by this workload were found.
      </Typography>
    );
  }

  const rows = hierarchyRows(resources, workload.metadata.uid);

  return (
    <SimpleTable
      columns={[
        { label: 'Kind', getter: (row: Row) => row.resource.kind },
        { label: 'Name', getter: (row: Row) => <NameCell row={row} /> },
        {
          label: 'Component',
          getter: (row: Row) =>
            row.resource.component
              ? `${row.resource.component}${row.resource.instance ? ` / ${row.resource.instance}` : ''}`
              : '',
        },
        { label: 'Status', getter: (row: Row) => row.resource.item.status?.phase ?? '' },
        {
          label: 'Age',
          getter: (row: Row) => age(row.resource.item.metadata.creationTimestamp),
        },
      ]}
      data={rows}
    />
  );
}
