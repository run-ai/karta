// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { describe, expect, it } from 'vitest';
import type { WorkloadTree } from '../api/tree';
import { buildFlowGraph, layoutFlowGraph } from './flowLayout';

const tree: WorkloadTree = {
  status: { phases: ['Running'] },
  children: [
    {
      name: 'service',
      hasPodDefinition: true,
      instances: [
        {
          instanceKey: 'Frontend',
          scale: { replicas: 2 },
          extractedInstance: {
            podTemplateSpec: {
              spec: {
                containers: [
                  { name: 'frontend', resources: { requests: { cpu: '2', memory: '4Gi' } } },
                ],
              },
            },
          },
        },
        {
          instanceKey: 'Worker',
          scale: { replicas: 8 },
          extractedInstance: {
            podTemplateSpec: {
              spec: {
                containers: [
                  {
                    name: 'worker',
                    resources: {
                      requests: { cpu: '8', memory: '32Gi' },
                      limits: { 'nvidia.com/gpu': '1' },
                    },
                  },
                ],
              },
            },
          },
        },
      ],
    },
  ],
};

describe('buildFlowGraph', () => {
  it('builds root -> component -> instance levels with unique ids', () => {
    const root = buildFlowGraph(tree, {
      kind: 'DynamoGraphDeployment',
      name: 'dynamo-llm',
    });

    expect(root.title).toBe('DynamoGraphDeployment');
    expect(root.subtitle).toBe('dynamo-llm');
    expect(root.phases).toEqual(['Running']);
    expect(root.children).toHaveLength(1);
    const component = root.children[0];
    expect(component.type).toBe('component');
    expect(component.title).toBe('service');
    expect(component.subtitle).toBe('logical group');
    expect(component.children.map(instance => instance.title)).toEqual(['Frontend', 'Worker']);

    const ids = new Set<string>();
    const walk = (node: typeof root) => {
      expect(ids.has(node.id)).toBe(false);
      ids.add(node.id);
      node.children.forEach(walk);
    };
    walk(root);
  });

  it('summarizes replicas and container resources on instances', () => {
    const root = buildFlowGraph(tree, { name: 'dynamo-llm' });
    const [frontend, worker] = root.children[0].children;
    expect(frontend.badge).toBe('x2');
    expect(frontend.containers).toEqual([{ name: 'frontend', cpu: '2', memory: '4Gi', gpu: '-' }]);
    expect(worker.containers).toEqual([{ name: 'worker', cpu: '8', memory: '32Gi', gpu: '1' }]);
  });

  it('carries empty phases when no phase matched', () => {
    const root = buildFlowGraph({ children: [] }, { name: 'w' });
    expect(root.phases).toEqual([]);
    expect(root.title).toBe('w');
    expect(root.subtitle).toBeUndefined();
  });

  it('attaches pods under their matching instance', () => {
    const root = buildFlowGraph(tree, { name: 'dynamo-llm' }, [
      { name: 'fe-0', phase: 'Running', component: 'service', instance: 'Frontend' },
      { name: 'wk-0', phase: 'Pending', component: 'service', instance: 'Worker' },
      { name: 'other', phase: 'Running', component: 'unknown' },
    ]);
    const [frontend, worker] = root.children[0].children;
    expect(frontend.children.map(pod => pod.title)).toEqual(['fe-0']);
    expect(frontend.children[0].type).toBe('pod');
    expect(frontend.children[0].phase).toBe('Running');
    expect(worker.children.map(pod => pod.title)).toEqual(['wk-0']);
  });

  it('attaches unkeyed pods to single-instance components and caps overflow', () => {
    const single: WorkloadTree = {
      children: [{ name: 'job', hasPodDefinition: true, instances: [{}] }],
    };
    const pods = Array.from({ length: 10 }, (_, index) => ({
      name: `pod-${index}`,
      component: 'job',
    }));
    const root = buildFlowGraph(single, { name: 'w' }, pods);
    const instance = root.children[0].children[0];
    expect(instance.children).toHaveLength(9);
    expect(instance.children[8].title).toBe('+2 more pods');
  });

  it('propagates links onto root and pod nodes', () => {
    const root = buildFlowGraph(tree, { kind: 'Dynamo', name: 'd', link: '/wl' }, [
      { name: 'fe-0', component: 'service', instance: 'Frontend', link: '/pods/fe-0' },
    ]);
    expect(root.link).toBe('/wl');
    expect(root.children[0].children[0].children[0].link).toBe('/pods/fe-0');
  });
});

describe('layoutFlowGraph', () => {
  const layout = layoutFlowGraph(buildFlowGraph(tree, { kind: 'Dynamo', name: 'dynamo-llm' }));

  it('places each level on its own row', () => {
    const rows = new Map<string, number>();
    for (const positioned of layout.nodes) {
      rows.set(positioned.node.type, positioned.y);
    }
    expect(rows.get('root')).toBeLessThan(rows.get('component')!);
    expect(rows.get('component')).toBeLessThan(rows.get('instance')!);
  });

  it('does not overlap siblings horizontally', () => {
    const instances = layout.nodes.filter(positioned => positioned.node.type === 'instance');
    expect(instances).toHaveLength(2);
    const [a, b] = [...instances].sort((left, right) => left.x - right.x);
    expect(a.x + a.width / 2).toBeLessThan(b.x - b.width / 2);
  });

  it('centers the parent over its children and connects them with edges', () => {
    const component = layout.nodes.find(positioned => positioned.node.type === 'component')!;
    const instances = layout.nodes.filter(positioned => positioned.node.type === 'instance');
    const childrenCenter =
      (Math.min(...instances.map(i => i.x - i.width / 2)) +
        Math.max(...instances.map(i => i.x + i.width / 2))) /
      2;
    expect(component.x).toBeCloseTo(childrenCenter, 5);
    expect(layout.edges).toHaveLength(3);
  });

  it('keeps every box inside the reported canvas', () => {
    for (const positioned of layout.nodes) {
      expect(positioned.x - positioned.width / 2).toBeGreaterThanOrEqual(0);
      expect(positioned.x + positioned.width / 2).toBeLessThanOrEqual(layout.width);
      expect(positioned.y + positioned.height).toBeLessThanOrEqual(layout.height);
    }
  });
});
