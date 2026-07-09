// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Kubernetes resource-quantity parsing, formatting, and workload aggregation.
// Kept free of React so it can be unit tested.

import type { ComponentNode, ExtractedInstance, WorkloadTree } from './tree';

export interface ResourceTotals {
  cpuMillis: number;
  memoryBytes: number;
  gpus: number;
}

export const EMPTY_TOTALS: ResourceTotals = { cpuMillis: 0, memoryBytes: 0, gpus: 0 };

// parseCpu converts a CPU quantity ("2", "100m", "12345678n") to millicores.
export function parseCpu(quantity?: string | number): number {
  if (quantity === undefined || quantity === null || quantity === '') {
    return 0;
  }
  const text = String(quantity);
  const match = text.match(/^([0-9.]+)(n|u|m)?$/);
  if (!match) {
    return 0;
  }
  const value = parseFloat(match[1]);
  switch (match[2]) {
    case 'n':
      return value / 1e6;
    case 'u':
      return value / 1e3;
    case 'm':
      return value;
    default:
      return value * 1000;
  }
}

const MEMORY_MULTIPLIERS: Record<string, number> = {
  Ki: 2 ** 10,
  Mi: 2 ** 20,
  Gi: 2 ** 30,
  Ti: 2 ** 40,
  Pi: 2 ** 50,
  k: 1e3,
  K: 1e3,
  M: 1e6,
  G: 1e9,
  T: 1e12,
  P: 1e15,
};

// parseMemory converts a memory quantity ("4Gi", "512Mi", "123456Ki", plain
// bytes) to bytes.
export function parseMemory(quantity?: string | number): number {
  if (quantity === undefined || quantity === null || quantity === '') {
    return 0;
  }
  const text = String(quantity);
  const match = text.match(/^([0-9.]+)([A-Za-z]+)?$/);
  if (!match) {
    return 0;
  }
  const value = parseFloat(match[1]);
  return value * (match[2] ? (MEMORY_MULTIPLIERS[match[2]] ?? 0) : 1);
}

function trimNumber(value: number): string {
  const fixed = value.toFixed(1);
  return fixed.endsWith('.0') ? fixed.slice(0, -2) : fixed;
}

export function formatCpu(cpuMillis: number): string {
  if (cpuMillis === 0) {
    return '0';
  }
  if (cpuMillis < 1000) {
    return `${Math.round(cpuMillis)}m`;
  }
  return trimNumber(cpuMillis / 1000);
}

export function formatMemory(memoryBytes: number): string {
  if (memoryBytes === 0) {
    return '0';
  }
  for (const [unit, size] of [
    ['Ti', 2 ** 40],
    ['Gi', 2 ** 30],
    ['Mi', 2 ** 20],
    ['Ki', 2 ** 10],
  ] as const) {
    if (memoryBytes >= size) {
      return `${trimNumber(memoryBytes / size)}${unit}`;
    }
  }
  return `${Math.round(memoryBytes)}`;
}

export function formatTotals(totals: ResourceTotals): string {
  return `cpu ${formatCpu(totals.cpuMillis)} · mem ${formatMemory(totals.memoryBytes)} · gpu ${totals.gpus}`;
}

export function addTotals(a: ResourceTotals, b: ResourceTotals): ResourceTotals {
  return {
    cpuMillis: a.cpuMillis + b.cpuMillis,
    memoryBytes: a.memoryBytes + b.memoryBytes,
    gpus: a.gpus + b.gpus,
  };
}

export function scaleTotals(totals: ResourceTotals, factor: number): ResourceTotals {
  return {
    cpuMillis: totals.cpuMillis * factor,
    memoryBytes: totals.memoryBytes * factor,
    gpus: totals.gpus * factor,
  };
}

interface ResourcesLike {
  requests?: Record<string, string | number>;
  limits?: Record<string, string | number>;
}

interface ContainerLike {
  resources?: ResourcesLike;
}

// containerRequests sums one container's requested resources. CPU and memory
// prefer requests; GPUs conventionally appear only in limits.
export function containerRequests(container: ContainerLike): ResourceTotals {
  const requests = container.resources?.requests ?? {};
  const limits = container.resources?.limits ?? {};
  return {
    cpuMillis: parseCpu(requests.cpu ?? limits.cpu),
    memoryBytes: parseMemory(requests.memory ?? limits.memory),
    gpus: Number(limits['nvidia.com/gpu'] ?? requests['nvidia.com/gpu'] ?? 0),
  };
}

function instanceContainers(instance?: ExtractedInstance): ContainerLike[] {
  const template = instance?.podTemplateSpec as { spec?: { containers?: ContainerLike[] } };
  const podSpec = instance?.podSpec as { containers?: ContainerLike[] } | undefined;
  const fragmented = instance?.fragmentedPodSpec as
    | { containers?: ContainerLike[]; container?: ContainerLike }
    | undefined;
  return (
    template?.spec?.containers ??
    podSpec?.containers ??
    fragmented?.containers ??
    (fragmented?.container ? [fragmented.container] : [])
  );
}

// aggregateTreeRequests sums the workload's desired resource requests from
// the spec tree: per instance, container requests multiplied by replicas.
export function aggregateTreeRequests(tree: WorkloadTree): ResourceTotals {
  function walkComponents(components: ComponentNode[]): ResourceTotals {
    let totals = EMPTY_TOTALS;
    for (const component of components) {
      for (const instance of component.instances ?? []) {
        const replicas = instance.scale?.replicas ?? 1;
        const perPod = instanceContainers(instance.extractedInstance).reduce(
          (sum, container) => addTotals(sum, containerRequests(container)),
          EMPTY_TOTALS
        );
        totals = addTotals(totals, scaleTotals(perPod, replicas));
        totals = addTotals(totals, walkComponents(instance.children ?? []));
      }
    }
    return totals;
  }
  return walkComponents(tree.children ?? []);
}
