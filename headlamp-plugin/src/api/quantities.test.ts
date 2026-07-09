// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { describe, expect, it } from 'vitest';
import {
  aggregateTreeRequests,
  formatCpu,
  formatMemory,
  formatTotals,
  parseCpu,
  parseMemory,
} from './quantities';
import type { WorkloadTree } from './tree';

describe('parseCpu', () => {
  it('parses cores, millicores, and nanocores', () => {
    expect(parseCpu('2')).toBe(2000);
    expect(parseCpu('100m')).toBe(100);
    expect(parseCpu('1500000000n')).toBe(1500);
    expect(parseCpu('0.5')).toBe(500);
    expect(parseCpu(undefined)).toBe(0);
  });
});

describe('parseMemory', () => {
  it('parses binary and decimal suffixes and plain bytes', () => {
    expect(parseMemory('1Gi')).toBe(2 ** 30);
    expect(parseMemory('512Mi')).toBe(512 * 2 ** 20);
    expect(parseMemory('128974Ki')).toBe(128974 * 2 ** 10);
    expect(parseMemory('1G')).toBe(1e9);
    expect(parseMemory('1024')).toBe(1024);
  });
});

describe('formatting', () => {
  it('formats cpu and memory human-readably', () => {
    expect(formatCpu(250)).toBe('250m');
    expect(formatCpu(14000)).toBe('14');
    expect(formatCpu(1500)).toBe('1.5');
    expect(formatMemory(64 * 2 ** 30)).toBe('64Gi');
    expect(formatMemory(1.5 * 2 ** 30)).toBe('1.5Gi');
  });
});

describe('aggregateTreeRequests', () => {
  const tree: WorkloadTree = {
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
                  containers: [{ resources: { requests: { cpu: '2', memory: '4Gi' } } }],
                },
              },
            },
          },
          {
            instanceKey: 'Worker',
            scale: { replicas: 3 },
            extractedInstance: {
              podTemplateSpec: {
                spec: {
                  containers: [
                    {
                      resources: {
                        requests: { cpu: '4', memory: '16Gi' },
                        limits: { 'nvidia.com/gpu': '2' },
                      },
                    },
                  ],
                },
              },
            },
            children: [
              {
                name: 'sidecar-group',
                hasPodDefinition: true,
                instances: [
                  {
                    extractedInstance: {
                      podSpec: {
                        containers: [{ resources: { requests: { cpu: '500m', memory: '1Gi' } } }],
                      },
                    },
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
  };

  it('multiplies per-pod requests by replicas and recurses into children', () => {
    const totals = aggregateTreeRequests(tree);
    // Frontend: 2 x (2 cpu, 4Gi); Worker: 3 x (4 cpu, 16Gi, 2 gpu);
    // nested sidecar (1 instance, default replicas 1, per Worker instance): 0.5 cpu, 1Gi.
    expect(totals.cpuMillis).toBe(2 * 2000 + 3 * 4000 + 500);
    expect(totals.memoryBytes).toBe((2 * 4 + 3 * 16 + 1) * 2 ** 30);
    expect(totals.gpus).toBe(6);
    expect(formatTotals(totals)).toBe('cpu 16.5 · mem 57Gi · gpu 6');
  });

  it('returns zeros for an empty tree', () => {
    const totals = aggregateTreeRequests({ children: [] });
    expect(formatTotals(totals)).toBe('cpu 0 · mem 0 · gpu 0');
  });
});
