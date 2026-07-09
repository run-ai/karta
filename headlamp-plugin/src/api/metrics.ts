// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Live pod usage from the metrics API (metrics-server). CPU and memory come
// from metrics.k8s.io; GPU usage is not reported there, so consumers derive
// GPU figures from the pods' allocations instead.

import { ApiProxy } from '@kinvolk/headlamp-plugin/lib';
import { useEffect, useState } from 'react';
import { parseCpu, parseMemory } from './quantities';

export interface PodUsage {
  cpuMillis: number;
  memoryBytes: number;
  /** False when the metrics API is not available in the cluster. */
  available: boolean;
}

interface PodMetrics {
  metadata: { name: string };
  containers?: { usage?: { cpu?: string; memory?: string } }[];
}

// usePodUsage sums live CPU/memory usage of the given pods. Returns null
// while loading.
export function usePodUsage(namespace: string | undefined, podNames: string[]): PodUsage | null {
  const [usage, setUsage] = useState<PodUsage | null>(null);
  const namesKey = podNames.slice().sort().join(',');

  useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!namespace || !podNames.length) {
        setUsage({ cpuMillis: 0, memoryBytes: 0, available: true });
        return;
      }
      const list = await ApiProxy.request(
        `/apis/metrics.k8s.io/v1beta1/namespaces/${namespace}/pods`
      ).catch(() => null);
      if (cancelled) {
        return;
      }
      if (!list?.items) {
        setUsage({ cpuMillis: 0, memoryBytes: 0, available: false });
        return;
      }

      const names = new Set(podNames);
      let cpuMillis = 0;
      let memoryBytes = 0;
      for (const item of list.items as PodMetrics[]) {
        if (!names.has(item.metadata.name)) {
          continue;
        }
        for (const container of item.containers ?? []) {
          cpuMillis += parseCpu(container.usage?.cpu);
          memoryBytes += parseMemory(container.usage?.memory);
        }
      }
      setUsage({ cpuMillis, memoryBytes, available: true });
    }

    setUsage(null);
    load();
    return () => {
      cancelled = true;
    };
  }, [namespace, namesKey]);

  return usage;
}
