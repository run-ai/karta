// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { KubeObject, KubeObjectInterface } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';

// Labels stamped on Karta objects by the operator, indexing the root GVK.
const LABEL_ROOT_GROUP = 'karta.run.ai/group';
const LABEL_ROOT_VERSION = 'karta.run.ai/version';
const LABEL_ROOT_KIND = 'karta.run.ai/kind';

export interface GroupVersionKind {
  group: string;
  version: string;
  kind: string;
}

interface KartaComponentDefinition {
  name: string;
  kind?: GroupVersionKind;
  ownerRef?: string;
}

export interface KartaCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
}

export interface KartaCRInterface extends KubeObjectInterface {
  spec: {
    structureDefinition?: {
      rootComponent?: KartaComponentDefinition;
      childComponents?: KartaComponentDefinition[];
      additionalChildKinds?: GroupVersionKind[];
    };
  };
  status?: {
    conditions?: KartaCondition[];
  };
}

export class KartaCR extends KubeObject<KartaCRInterface> {
  static kind = 'Karta';
  static apiName = 'kartas';
  static apiVersion = 'run.ai/v1alpha1';
  static isNamespaced = false;

  get rootGVK(): GroupVersionKind | null {
    const labels = this.metadata?.labels ?? {};
    const version = labels[LABEL_ROOT_VERSION];
    const kind = labels[LABEL_ROOT_KIND];
    if (version && kind) {
      // The group label is absent for the core API group.
      return { group: labels[LABEL_ROOT_GROUP] ?? '', version, kind };
    }
    const specKind = this.jsonData?.spec?.structureDefinition?.rootComponent?.kind;
    if (specKind?.version && specKind?.kind) {
      return { group: specKind.group ?? '', version: specKind.version, kind: specKind.kind };
    }
    return null;
  }

  get childComponentNames(): string[] {
    const children = this.jsonData?.spec?.structureDefinition?.childComponents ?? [];
    return children.map(child => child.name);
  }

  // readyCondition returns the operator-managed Ready condition, which also
  // covers spec validation and target CRD existence. Null when the operator
  // has not reconciled this Karta.
  get readyCondition(): KartaCondition | null {
    const conditions = this.jsonData?.status?.conditions ?? [];
    return conditions.find(condition => condition.type === 'Ready') ?? null;
  }

  // childKinds returns the distinct kinds of Kubernetes objects this workload
  // creates: component kinds plus additionalChildKinds from the definition.
  get childKinds(): GroupVersionKind[] {
    const structure = this.jsonData?.spec?.structureDefinition;
    const kinds = [
      ...(structure?.childComponents ?? []).map(child => child.kind),
      ...(structure?.additionalChildKinds ?? []),
    ];
    const seen = new Set<string>();
    const result: GroupVersionKind[] = [];
    for (const kind of kinds) {
      if (!kind?.kind || !kind?.version) {
        continue;
      }
      const key = `${kind.group ?? ''}/${kind.version}/${kind.kind}`;
      if (!seen.has(key)) {
        seen.add(key);
        result.push({ group: kind.group ?? '', version: kind.version, kind: kind.kind });
      }
    }
    return result;
  }
}
