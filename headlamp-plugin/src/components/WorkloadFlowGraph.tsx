// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import '@xyflow/react/dist/style.css';
import {
  alpha,
  Box,
  Chip,
  Divider,
  FormControlLabel,
  Link as MuiLink,
  Paper,
  Switch,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
import type { Edge, Node, NodeProps } from '@xyflow/react';
import { Background, Controls, Handle, MarkerType, Position, ReactFlow } from '@xyflow/react';
import { useMemo, useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import type { WorkloadTree } from '../api/tree';
import type { ContainerSummary, FlowNode, FlowNodeType, PodInfo, RootLabel } from './flowLayout';
import { buildFlowGraph, layoutFlowGraph } from './flowLayout';
import StatusPhaseChips from './StatusPhaseChips';

type KartaNodeData = { flowNode: FlowNode; width: number };
type KartaFlowNode = Node<KartaNodeData, 'karta'>;

const POD_PHASE_KEY: Record<string, 'success' | 'info' | 'error' | 'warning'> = {
  Running: 'success',
  Succeeded: 'info',
  Failed: 'error',
  Pending: 'warning',
};

// useTypeAccents returns the accent color per node type. Component purple is
// a fixed pair (deepPurple 400/300) because theme palettes have no purple
// slot that is stable across themes.
function useTypeAccents(): Record<Exclude<FlowNodeType, 'pod'>, string> {
  const theme = useTheme();
  return {
    root: theme.palette.info.main,
    component: theme.palette.mode === 'dark' ? '#b39ddb' : '#7e57c2',
    instance: theme.palette.success.main,
  };
}

function useAccentColor(node: FlowNode): string {
  const theme = useTheme();
  const accents = useTypeAccents();
  if (node.type === 'pod') {
    const key = node.phase ? POD_PHASE_KEY[node.phase] : undefined;
    return key ? theme.palette[key].main : theme.palette.grey[500];
  }
  return accents[node.type];
}

function NodeTitle({ flowNode }: { flowNode: FlowNode }) {
  const title = (
    <Typography variant="subtitle2" noWrap sx={{ minWidth: 0 }}>
      {flowNode.title}
    </Typography>
  );
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
      {flowNode.link ? (
        <Tooltip title="Open resource">
          <MuiLink
            component={RouterLink}
            to={flowNode.link}
            underline="hover"
            color="inherit"
            sx={{ minWidth: 0 }}
          >
            {title}
          </MuiLink>
        </Tooltip>
      ) : (
        title
      )}
      {flowNode.badge && (
        <Chip size="small" variant="outlined" label={flowNode.badge} sx={{ height: 18 }} />
      )}
    </Box>
  );
}

function ContainerRow({ container }: { container: ContainerSummary }) {
  const theme = useTheme();
  const hasGpu = container.gpu !== '-' && container.gpu !== '0';
  return (
    <Box sx={{ lineHeight: 1.2 }}>
      <Typography variant="caption" sx={{ fontWeight: 600 }}>
        {container.name}
      </Typography>
      <Typography variant="caption" display="block" color="text.secondary">
        cpu {container.cpu} · mem {container.memory} ·{' '}
        <Box
          component="span"
          sx={hasGpu ? { color: theme.palette.success.main, fontWeight: 600 } : undefined}
        >
          gpu {container.gpu}
        </Box>
      </Typography>
    </Box>
  );
}

function KartaNode({ data }: NodeProps<KartaFlowNode>) {
  const { flowNode, width } = data;
  const accent = useAccentColor(flowNode);

  return (
    <Paper
      elevation={2}
      sx={{
        width,
        px: 1.5,
        py: 1,
        borderLeft: `4px solid ${accent}`,
        borderRadius: 1,
        backgroundImage: theme =>
          `linear-gradient(${alpha(accent, theme.palette.mode === 'dark' ? 0.08 : 0.04)}, ${alpha(
            accent,
            theme.palette.mode === 'dark' ? 0.08 : 0.04
          )})`,
      }}
    >
      <Handle type="target" position={Position.Top} isConnectable={false} style={{ opacity: 0 }} />
      <NodeTitle flowNode={flowNode} />
      {flowNode.subtitle && (
        <Typography variant="caption" color="text.secondary" display="block" noWrap>
          {flowNode.subtitle}
        </Typography>
      )}
      {flowNode.phases && (
        <Box sx={{ mt: 0.5 }}>
          <StatusPhaseChips phases={flowNode.phases} />
        </Box>
      )}
      {flowNode.containers && flowNode.containers.length > 0 && (
        <>
          <Divider sx={{ my: 0.5 }} />
          {flowNode.containers.map(container => (
            <ContainerRow key={container.name} container={container} />
          ))}
        </>
      )}
      {flowNode.details.map(detail => (
        <Typography key={detail} variant="caption" display="block" color="text.secondary" noWrap>
          {detail}
        </Typography>
      ))}
      <Handle
        type="source"
        position={Position.Bottom}
        isConnectable={false}
        style={{ opacity: 0 }}
      />
    </Paper>
  );
}

const nodeTypes = { karta: KartaNode };

function Legend({ hasPods }: { hasPods: boolean }) {
  const theme = useTheme();
  const accents = useTypeAccents();
  const items: { color: string; label: string }[] = [
    { color: accents.root, label: 'workload' },
    { color: accents.component, label: 'component' },
    { color: accents.instance, label: 'instance' },
    ...(hasPods ? [{ color: theme.palette.grey[500], label: 'pod (colored by phase)' }] : []),
  ];
  return (
    <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
      {items.map(item => (
        <Box key={item.label} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <Box sx={{ width: 12, height: 12, borderRadius: 0.5, backgroundColor: item.color }} />
          <Typography variant="caption">{item.label}</Typography>
        </Box>
      ))}
    </Box>
  );
}

// WorkloadFlowGraph renders the tree as an interactive top-down flow graph
// (pan, zoom, draggable nodes): the workload root, its components, their
// instances with scale and container resources, and the live pods.
export default function WorkloadFlowGraph({
  tree,
  rootLabel,
  pods = [],
}: {
  tree: WorkloadTree;
  rootLabel: RootLabel;
  pods?: PodInfo[];
}) {
  const theme = useTheme();
  const [showPods, setShowPods] = useState(true);
  const shownPods = showPods ? pods : [];

  const { nodes, edges, height } = useMemo(() => {
    const layout = layoutFlowGraph(buildFlowGraph(tree, rootLabel, shownPods));
    const nodes: KartaFlowNode[] = layout.nodes.map(positioned => ({
      id: positioned.node.id,
      type: 'karta',
      position: { x: positioned.x - positioned.width / 2, y: positioned.y },
      data: { flowNode: positioned.node, width: positioned.width },
    }));
    const edges: Edge[] = layout.edges.map(edge => ({
      id: `${edge.from.node.id}->${edge.to.node.id}`,
      source: edge.from.node.id,
      target: edge.to.node.id,
      type: 'smoothstep',
      markerEnd: { type: MarkerType.ArrowClosed },
    }));
    return { nodes, edges, height: layout.height };
  }, [tree, rootLabel.kind, rootLabel.name, rootLabel.link, shownPods]);

  return (
    <Box>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          mb: 1,
        }}
      >
        <Legend hasPods={showPods && pods.length > 0} />
        {pods.length > 0 && (
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={showPods}
                onChange={event => setShowPods(event.target.checked)}
              />
            }
            label={<Typography variant="caption">{`Pods (${pods.length})`}</Typography>}
          />
        )}
      </Box>
      <Box
        sx={{
          height: Math.min(Math.max(height + 120, 320), 720),
          border: `1px solid ${theme.palette.divider}`,
          borderRadius: 1,
        }}
      >
        <ReactFlow
          key={showPods ? 'with-pods' : 'without-pods'}
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          colorMode={theme.palette.mode}
          fitView
          fitViewOptions={{ padding: 0.1, maxZoom: 1 }}
          minZoom={0.15}
          nodesConnectable={false}
        >
          <Background gap={16} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </Box>
    </Box>
  );
}
