// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import type { KubeObject } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { useEffect, useState } from 'react';
import type { GroupVersionKind } from '../resources/karta';
import { getTreeEngine } from './wasmEngine';

// Wire format of the WorkloadTree JSON produced by pkg/tree in the Karta
// repo. Only the fields the UI consumes are typed strictly.
export interface Scale {
  replicas?: number;
  minReplicas?: number;
  maxReplicas?: number;
}

export interface ExtractedInstance {
  podTemplateSpec?: Record<string, unknown>;
  podSpec?: Record<string, unknown>;
  fragmentedPodSpec?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  scale?: Scale;
}

export interface InstanceNode {
  instanceKey?: string;
  replicaKey?: string;
  scale?: Scale;
  extractedInstance?: ExtractedInstance;
  children?: ComponentNode[];
}

export interface ComponentNode {
  name: string;
  kind?: GroupVersionKind;
  hasPodDefinition: boolean;
  instances?: InstanceNode[];
}

export interface WorkloadTree {
  status?: {
    phases?: string[];
  };
  children?: ComponentNode[];
}

export class TreeApiError extends Error {}

// fetchWorkloadTree evaluates the Karta definition against the workload with
// the WebAssembly tree engine bundled in the plugin; nothing leaves the
// browser.
export async function fetchWorkloadTree(
  karta: KubeObject,
  workload: KubeObject
): Promise<WorkloadTree> {
  let engine;
  try {
    engine = await getTreeEngine();
  } catch (err) {
    const message = (err as Error)?.message || String(err);
    throw new TreeApiError(
      `Failed to load the Karta tree engine (${message}). The plugin build may be missing its ` +
        `WebAssembly module; rebuild it with "make plugin-wasm".`
    );
  }

  const result = engine.buildTree(
    JSON.stringify(karta.jsonData),
    JSON.stringify(workload.jsonData)
  );
  if (result.error || !result.tree) {
    throw new TreeApiError(result.error ?? 'the tree engine returned no tree');
  }
  return JSON.parse(result.tree) as WorkloadTree;
}

export interface PodMatch {
  component: string;
  instance?: string;
}

// matchPodsToComponents classifies pods with the definition's podSelector:
// componentTypeSelector gives the component, componentInstanceSelector (when
// defined) the instance key. The caller must already have scoped the pods to
// a single workload.
export async function matchPodsToComponents(
  karta: KubeObject,
  pods: unknown[]
): Promise<Record<string, PodMatch>> {
  const engine = await getTreeEngine();
  const result = engine.matchPods(JSON.stringify(karta.jsonData), JSON.stringify(pods));
  if (result.error) {
    throw new TreeApiError(result.error);
  }
  return result.matches ?? {};
}

export interface UseWorkloadTreeResult {
  tree: WorkloadTree | null;
  error: TreeApiError | null;
  loading: boolean;
}

export function useWorkloadTree(
  karta: KubeObject | null,
  workload: KubeObject | null
): UseWorkloadTreeResult {
  const [result, setResult] = useState<UseWorkloadTreeResult>({
    tree: null,
    error: null,
    loading: true,
  });

  const kartaVersion = karta?.metadata?.resourceVersion;
  const workloadVersion = workload?.metadata?.resourceVersion;

  useEffect(() => {
    if (!karta || !workload) {
      return;
    }
    let cancelled = false;
    setResult({ tree: null, error: null, loading: true });
    fetchWorkloadTree(karta, workload)
      .then(tree => {
        if (!cancelled) {
          setResult({ tree, error: null, loading: false });
        }
      })
      .catch((error: TreeApiError) => {
        if (!cancelled) {
          setResult({ tree: null, error, loading: false });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [karta?.metadata?.uid, kartaVersion, workload?.metadata?.uid, workloadVersion]);

  return result;
}
