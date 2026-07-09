// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Pure graph-building and layout logic for the workload flow graph. Kept free
// of React so it can be unit tested.

import type { ComponentNode, ExtractedInstance, InstanceNode, WorkloadTree } from '../api/tree';

export type FlowNodeType = 'root' | 'component' | 'instance' | 'pod';

export interface ContainerSummary {
  name: string;
  cpu: string;
  memory: string;
  gpu: string;
}

export interface FlowNode {
  id: string;
  type: FlowNodeType;
  title: string;
  subtitle?: string;
  /** Small chip next to the title, e.g. replica count. */
  badge?: string;
  /** Normalized status phases; only set on the root node. */
  phases?: string[];
  /** Per-container resource summaries; only set on instance nodes. */
  containers?: ContainerSummary[];
  /** Extra caption lines (min-max, replica key, pod node name). */
  details: string[];
  /** Pod phase; only set on pod nodes, drives the accent color. */
  phase?: string;
  /** Href to the resource's Headlamp page. */
  link?: string;
  children: FlowNode[];
}

/** A live pod already classified to a component (and optionally instance). */
export interface PodInfo {
  name: string;
  phase?: string;
  nodeName?: string;
  component: string;
  instance?: string;
  link?: string;
}

export interface PositionedNode {
  node: FlowNode;
  // x is the horizontal center of the box; y is its top edge.
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface FlowEdge {
  from: PositionedNode;
  to: PositionedNode;
}

export interface FlowLayout {
  nodes: PositionedNode[];
  edges: FlowEdge[];
  width: number;
  height: number;
}

const CHAR_WIDTH = 7.5;
const DETAIL_LINE_HEIGHT = 18;
const TITLE_HEIGHT = 24;
const SUBTITLE_HEIGHT = 18;
const PHASES_HEIGHT = 30;
const CONTAINER_ROW_HEIGHT = 30;
const PAD_X = 16;
const PAD_Y = 10;
const MIN_BOX_WIDTH = 160;
const MAX_PODS_PER_INSTANCE = 8;
const GAP_X = 32;
const GAP_Y = 56;
const MARGIN = 12;

interface Resources {
  cpu?: string | number;
  memory?: string | number;
  'nvidia.com/gpu'?: string | number;
}

interface ContainerLike {
  name?: string;
  resources?: { requests?: Resources; limits?: Resources };
}

// containerSummaries extracts per-container resources. CPU and memory prefer
// requests; GPUs conventionally appear only in limits, so limits are
// preferred there.
function containerSummaries(instance?: ExtractedInstance): ContainerSummary[] {
  const template = instance?.podTemplateSpec as { spec?: { containers?: ContainerLike[] } };
  const podSpec = instance?.podSpec as { containers?: ContainerLike[] } | undefined;
  const fragmented = instance?.fragmentedPodSpec as
    | { containers?: ContainerLike[]; container?: ContainerLike }
    | undefined;
  const containers =
    template?.spec?.containers ??
    podSpec?.containers ??
    fragmented?.containers ??
    (fragmented?.container ? [fragmented.container] : undefined);
  if (!containers?.length) {
    return [];
  }
  return containers.map(container => {
    const requests = container.resources?.requests ?? {};
    const limits = container.resources?.limits ?? {};
    return {
      name: container.name ?? 'container',
      cpu: String(requests.cpu ?? limits.cpu ?? '-'),
      memory: String(requests.memory ?? limits.memory ?? '-'),
      gpu: String(limits['nvidia.com/gpu'] ?? requests['nvidia.com/gpu'] ?? '-'),
    };
  });
}

function podFlowNode(pod: PodInfo, parentId: string): FlowNode {
  return {
    id: `${parentId}/pod:${pod.name}`,
    type: 'pod',
    title: pod.name,
    subtitle: pod.phase,
    phase: pod.phase,
    details: pod.nodeName ? [`node: ${pod.nodeName}`] : [],
    link: pod.link,
    children: [],
  };
}

// podsForInstance selects the pods that belong to this instance: the ones
// whose extracted instance key matches, or all component pods when the
// component is single-instance.
function podsForInstance(
  pods: PodInfo[],
  componentName: string,
  instanceKey: string | undefined,
  instanceCount: number
): PodInfo[] {
  return pods.filter(pod => {
    if (pod.component !== componentName) {
      return false;
    }
    if (pod.instance !== undefined && instanceKey !== undefined) {
      return pod.instance === instanceKey;
    }
    return instanceCount === 1;
  });
}

function instanceFlowNode(
  instance: InstanceNode,
  index: number,
  component: ComponentNode,
  parentId: string,
  pods: PodInfo[]
): FlowNode {
  const id = `${parentId}/instance:${instance.instanceKey ?? index}`;

  let badge: string | undefined;
  if (instance.scale?.replicas !== undefined) {
    badge = `x${instance.scale.replicas}`;
  }
  const details: string[] = [];
  if (instance.replicaKey) {
    details.push(`replica: ${instance.replicaKey}`);
  }
  if (instance.scale?.minReplicas !== undefined || instance.scale?.maxReplicas !== undefined) {
    details.push(
      `scale ${instance.scale?.minReplicas ?? '-'}..${instance.scale?.maxReplicas ?? '-'}`
    );
  }

  const instancePods = podsForInstance(
    pods,
    component.name,
    instance.instanceKey,
    component.instances?.length ?? 1
  );
  const shownPods = instancePods.slice(0, MAX_PODS_PER_INSTANCE).map(pod => podFlowNode(pod, id));
  if (instancePods.length > MAX_PODS_PER_INSTANCE) {
    shownPods.push({
      id: `${id}/pod:overflow`,
      type: 'pod',
      title: `+${instancePods.length - MAX_PODS_PER_INSTANCE} more pods`,
      details: [],
      children: [],
    });
  }

  return {
    id,
    type: 'instance',
    title: instance.instanceKey ?? 'default',
    badge,
    containers: containerSummaries(instance.extractedInstance),
    details,
    children: [
      ...(instance.children ?? []).map(child => componentFlowNode(child, id, pods)),
      ...shownPods,
    ],
  };
}

function componentFlowNode(component: ComponentNode, parentId: string, pods: PodInfo[]): FlowNode {
  const id = `${parentId}/component:${component.name}`;
  return {
    id,
    type: 'component',
    title: component.name,
    subtitle: component.kind ? component.kind.kind : 'logical group',
    details: [],
    children: (component.instances ?? []).map((instance, index) =>
      instanceFlowNode(instance, index, component, id, pods)
    ),
  };
}

export interface RootLabel {
  kind?: string;
  name: string;
  link?: string;
}

export function buildFlowGraph(tree: WorkloadTree, root: RootLabel, pods: PodInfo[] = []): FlowNode {
  return {
    id: 'root',
    type: 'root',
    title: root.kind ?? root.name,
    subtitle: root.kind ? root.name : undefined,
    details: [],
    phases: tree.status?.phases ?? [],
    link: root.link,
    children: (tree.children ?? []).map(child => componentFlowNode(child, 'root', pods)),
  };
}

function containerRowText(container: ContainerSummary): string {
  return `${container.name}  cpu ${container.cpu} · mem ${container.memory} · gpu ${container.gpu}`;
}

export function boxSize(node: FlowNode): { width: number; height: number } {
  const texts = [
    node.title + (node.badge ? node.badge.length + 3 : 0),
    node.subtitle ?? '',
    ...node.details,
    ...(node.containers ?? []).map(containerRowText),
  ];
  const longest = texts.reduce(
    (max, line) => Math.max(max, typeof line === 'string' ? line.length : 0),
    0
  );
  let height = PAD_Y * 2 + TITLE_HEIGHT;
  if (node.subtitle) {
    height += SUBTITLE_HEIGHT;
  }
  if (node.phases) {
    height += PHASES_HEIGHT;
  }
  height += (node.containers?.length ?? 0) * CONTAINER_ROW_HEIGHT;
  height += node.details.length * DETAIL_LINE_HEIGHT;
  return {
    width: Math.max(MIN_BOX_WIDTH, longest * CHAR_WIDTH + PAD_X * 2),
    height,
  };
}

// layoutFlowGraph assigns positions: nodes of the same depth share a row and
// every parent is centered above the span of its children.
export function layoutFlowGraph(root: FlowNode): FlowLayout {
  const subtreeWidths = new Map<FlowNode, number>();

  function measureSubtree(node: FlowNode): number {
    const childrenWidth = node.children.reduce(
      (sum, child, index) => sum + measureSubtree(child) + (index > 0 ? GAP_X : 0),
      0
    );
    const width = Math.max(boxSize(node).width, childrenWidth);
    subtreeWidths.set(node, width);
    return width;
  }
  measureSubtree(root);

  const levelHeights: number[] = [];
  function collectLevelHeights(node: FlowNode, depth: number) {
    levelHeights[depth] = Math.max(levelHeights[depth] ?? 0, boxSize(node).height);
    node.children.forEach(child => collectLevelHeights(child, depth + 1));
  }
  collectLevelHeights(root, 0);

  const levelTops: number[] = [];
  let top = MARGIN;
  for (const height of levelHeights) {
    levelTops.push(top);
    top += height + GAP_Y;
  }

  const nodes: PositionedNode[] = [];
  const edges: FlowEdge[] = [];

  function place(node: FlowNode, left: number, depth: number): PositionedNode {
    const span = subtreeWidths.get(node) ?? 0;
    const { width, height } = boxSize(node);
    const positioned: PositionedNode = {
      node,
      x: left + span / 2,
      y: levelTops[depth],
      width,
      height,
    };
    nodes.push(positioned);

    const childrenSpan = node.children.reduce(
      (sum, child, index) => sum + (subtreeWidths.get(child) ?? 0) + (index > 0 ? GAP_X : 0),
      0
    );
    let childLeft = left + (span - childrenSpan) / 2;
    for (const child of node.children) {
      const childPositioned = place(child, childLeft, depth + 1);
      edges.push({ from: positioned, to: childPositioned });
      childLeft += (subtreeWidths.get(child) ?? 0) + GAP_X;
    }
    return positioned;
  }
  place(root, MARGIN, 0);

  return {
    nodes,
    edges,
    width: (subtreeWidths.get(root) ?? 0) + MARGIN * 2,
    height: top - GAP_Y + MARGIN,
  };
}
