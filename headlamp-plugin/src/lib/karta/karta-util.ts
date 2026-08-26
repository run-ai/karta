// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Envelope, getKartaEngine } from './karta';
import { Karta, Pod, PodAttribution, Workload, WorkloadTree } from './karta.types';

function unwrap<T>(envelope: Envelope, fallback: T): T {
  if (envelope.error !== null) {
    throw new Error(envelope.error);
  }
  if (envelope.data === null) {
    return fallback;
  }
  return JSON.parse(envelope.data) as T;
}

export async function buildTree(definition: Karta, workload: Workload): Promise<WorkloadTree> {
  const engine = await getKartaEngine();
  return unwrap(engine.buildTree(JSON.stringify(definition), JSON.stringify(workload)), {
    Status: null,
    Children: [],
  });
}

export async function attributePods(definition: Karta, workload: Workload, pods: Pod[]): Promise<PodAttribution[]> {
  const engine = await getKartaEngine();
  return unwrap(
    engine.attributePods(JSON.stringify(definition), JSON.stringify(workload), JSON.stringify(pods)),
    []
  );
}

export async function evaluatePhases(definition: Karta, workload: Workload): Promise<string[]> {
  const engine = await getKartaEngine();
  return unwrap(engine.evaluatePhases(JSON.stringify(definition), JSON.stringify(workload)), []);
}

export async function listCatalog(): Promise<Karta[]> {
  const engine = await getKartaEngine();
  return unwrap(engine.listCatalog(), []);
}
