// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Icon } from '@iconify/react';
import { Loader } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import type { KubeObject } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Chip,
  Typography,
} from '@mui/material';
import type { ComponentNode, InstanceNode, WorkloadTree } from '../api/tree';
import { useWorkloadTree } from '../api/tree';
import type { PodInfo } from './flowLayout';
import StatusPhaseChips from './StatusPhaseChips';
import WorkloadFlowGraph from './WorkloadFlowGraph';

function formatScale(scale?: InstanceNode['scale']): string | null {
  if (!scale) {
    return null;
  }
  const parts: string[] = [];
  if (scale.replicas !== undefined) {
    parts.push(`replicas: ${scale.replicas}`);
  }
  if (scale.minReplicas !== undefined || scale.maxReplicas !== undefined) {
    parts.push(`min-max: ${scale.minReplicas ?? '-'}..${scale.maxReplicas ?? '-'}`);
  }
  return parts.length ? parts.join(', ') : null;
}

function formatKind(kind?: ComponentNode['kind']): string {
  if (!kind) {
    return 'logical group';
  }
  const groupVersion = kind.group ? `${kind.group}/${kind.version}` : kind.version;
  return `${groupVersion} ${kind.kind}`;
}

function InstanceView({ instance, depth }: { instance: InstanceNode; depth: number }) {
  const scale = formatScale(instance.scale);
  const image = (
    instance.extractedInstance?.podTemplateSpec as
      | { spec?: { containers?: { image?: string }[] } }
      | undefined
  )?.spec?.containers?.[0]?.image;

  return (
    <Box sx={{ ml: 2, borderLeft: '1px solid', borderColor: 'divider', pl: 2, py: 0.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Icon icon="mdi:cube-outline" />
        <Typography variant="body2">
          {instance.instanceKey ?? 'default'}
          {instance.replicaKey ? ` / replica ${instance.replicaKey}` : ''}
        </Typography>
        {scale && <Chip size="small" variant="outlined" label={scale} />}
        {image && <Chip size="small" variant="outlined" label={image} />}
      </Box>
      {instance.children?.map(child => (
        <ComponentView key={child.name} component={child} depth={depth + 1} />
      ))}
    </Box>
  );
}

function ComponentView({ component, depth }: { component: ComponentNode; depth: number }) {
  return (
    <Accordion defaultExpanded={depth < 2} disableGutters elevation={0} square>
      <AccordionSummary expandIcon={<Icon icon="mdi:chevron-down" />}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
          <Icon icon={component.hasPodDefinition ? 'mdi:cube-send' : 'mdi:folder-outline'} />
          <Typography variant="subtitle2">{component.name}</Typography>
          <Typography variant="caption" color="text.secondary">
            {formatKind(component.kind)}
          </Typography>
          {component.hasPodDefinition && (
            <Chip size="small" variant="outlined" label="pod definition" />
          )}
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ py: 0 }}>
        {component.instances?.map((instance, index) => (
          <InstanceView key={instance.instanceKey ?? index} instance={instance} depth={depth} />
        ))}
      </AccordionDetails>
    </Accordion>
  );
}

export function TreeContent({ tree }: { tree: WorkloadTree }) {
  if (!tree.children?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        The workload has no child components.
      </Typography>
    );
  }
  return (
    <Box>
      {tree.children.map(component => (
        <ComponentView key={component.name} component={component} depth={0} />
      ))}
    </Box>
  );
}

// WorkloadTreePanel fetches and renders the tree for one workload. It is
// shared between the full tree page (flow chart) and the map node details
// panel, whose narrow width calls for the compact list variant.
export default function WorkloadTreePanel({
  karta,
  workload,
  variant = 'flow',
  pods,
  rootLink,
}: {
  karta: KubeObject | null;
  workload: KubeObject | null;
  variant?: 'flow' | 'list';
  pods?: PodInfo[];
  rootLink?: string;
}) {
  const { tree, error, loading } = useWorkloadTree(karta, workload);

  if (error) {
    return <Alert severity="error">{error.message}</Alert>;
  }
  if (loading || !tree || !workload) {
    return <Loader title="Loading workload tree" />;
  }

  if (variant === 'flow') {
    return (
      <WorkloadFlowGraph
        tree={tree}
        rootLabel={{
          kind: (workload.jsonData as { kind?: string })?.kind,
          name: workload.getName(),
          link: rootLink,
        }}
        pods={pods}
      />
    );
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <Typography variant="subtitle1">Status</Typography>
        <StatusPhaseChips phases={tree.status?.phases} />
      </Box>
      <TreeContent tree={tree} />
    </Box>
  );
}
