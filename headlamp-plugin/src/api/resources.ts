// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Discovery of the live Kubernetes objects belonging to a workload: the child
// kinds from the Karta definition plus Pods, scoped by ownerReferences with a
// podSelector fallback for pods created without owner chains.

import { ApiProxy, K8s, Router } from '@kinvolk/headlamp-plugin/lib';
import type { KubeObject, KubeObjectClass } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { useEffect, useState } from 'react';
import type { GroupVersionKind } from '../resources/karta';
import { KartaCR } from '../resources/karta';
import { matchPodsToComponents } from './tree';

export interface SubResourceItem {
  metadata: {
    uid: string;
    name: string;
    namespace?: string;
    creationTimestamp?: string;
    ownerReferences?: { uid: string }[];
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: {
    nodeName?: string;
    containers?: {
      resources?: {
        requests?: Record<string, string | number>;
        limits?: Record<string, string | number>;
      };
    }[];
  };
  status?: { phase?: string };
}

export interface SubResource {
  kind: string;
  item: SubResourceItem;
  /** CRD metadata.name (plural.group) when the kind is CRD-backed. */
  crdName?: string;
  /** Component name from the Karta podSelector (pods only). */
  component?: string;
  /** Instance key from the Karta componentInstanceSelector (pods only). */
  instance?: string;
  /** Uid of the owner within the workload's object set. */
  parentUid?: string;
  /** Href to the resource's Headlamp page. */
  href?: string;
}

interface CrdInfo {
  metadata: { name: string };
  spec: { group: string; names: { kind: string; plural: string }; scope: string };
}

function listPath(gvk: GroupVersionKind, plural: string, namespace?: string): string {
  const prefix = gvk.group ? `/apis/${gvk.group}/${gvk.version}` : `/api/${gvk.version}`;
  return namespace ? `${prefix}/namespaces/${namespace}/${plural}` : `${prefix}/${plural}`;
}

// Pods are always fetched: every pod-bearing workload produces them, and they
// are what users drill into most.
const POD_GVK: GroupVersionKind = { group: '', version: 'v1', kind: 'Pod' };

function resolvePlural(
  gvk: GroupVersionKind,
  crds: CrdInfo[]
): { plural?: string; crdName?: string } {
  const crd = crds.find(c => c.spec.group === gvk.group && c.spec.names.kind === gvk.kind);
  if (crd) {
    return { plural: crd.spec.names.plural, crdName: crd.metadata.name };
  }
  const builtin = (K8s.ResourceClasses as Record<string, KubeObjectClass>)[gvk.kind];
  if (builtin) {
    return { plural: builtin.apiName };
  }
  return {};
}

export function subResourceHref(resource: SubResource): string | undefined {
  const builtin = (K8s.ResourceClasses as Record<string, KubeObjectClass>)[resource.kind];
  if (builtin) {
    try {
      return new builtin(resource.item as any).getDetailsLink();
    } catch {
      return undefined;
    }
  }
  if (resource.crdName) {
    return Router.createRouteURL('customresource', {
      crd: resource.crdName,
      namespace: resource.item.metadata.namespace ?? '-',
      crName: resource.item.metadata.name,
    });
  }
  return undefined;
}

// useWorkloadResources lists the workload's child kinds (from the Karta
// definition, plus Pods) in the workload's namespace and keeps the objects
// transitively owned by the workload. Pods are additionally classified with
// the definition's podSelector; pods without an owner chain are kept only
// when the selector matches and the pod references the workload by name.
export function useWorkloadResources(
  karta: KartaCR,
  workload: KubeObject
): SubResource[] | null {
  const [resources, setResources] = useState<SubResource[] | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      const namespace = workload.getNamespace();
      const kinds = [...karta.childKinds, POD_GVK];

      const crdList = await ApiProxy.request(
        '/apis/apiextensions.k8s.io/v1/customresourcedefinitions'
      ).catch(() => null);
      const crds: CrdInfo[] = crdList?.items ?? [];

      const candidates: SubResource[] = [];
      await Promise.all(
        kinds.map(async gvk => {
          const { plural, crdName } = resolvePlural(gvk, crds);
          if (!plural) {
            return;
          }
          const list = await ApiProxy.request(listPath(gvk, plural, namespace)).catch(() => null);
          for (const item of list?.items ?? []) {
            candidates.push({ kind: gvk.kind, item, crdName });
          }
        })
      );

      // Keep objects whose ownerReferences chain reaches the workload, and
      // remember which owner in the set each object hangs off.
      const owned = new Set<string>([workload.metadata.uid]);
      const parents = new Map<string, string>();
      let grew = true;
      while (grew) {
        grew = false;
        for (const candidate of candidates) {
          const uid = candidate.item.metadata.uid;
          if (owned.has(uid)) {
            continue;
          }
          const ownerRef = candidate.item.metadata.ownerReferences?.find(ref => owned.has(ref.uid));
          if (ownerRef) {
            owned.add(uid);
            parents.set(uid, ownerRef.uid);
            grew = true;
          }
        }
      }
      const result = candidates.filter(candidate => owned.has(candidate.item.metadata.uid));

      // Fallback for pods without an owner chain to the workload (some
      // operators create pods without ownerReferences). The selector
      // identifies component membership, not workload identity, so only pods
      // that also reference the workload by name are considered.
      const orphanPods = candidates.filter(
        candidate => candidate.kind === 'Pod' && !owned.has(candidate.item.metadata.uid)
      );
      const workloadScoped = orphanPods.filter(candidate => {
        const meta = candidate.item.metadata;
        const values = [
          ...Object.values(meta.labels ?? {}),
          ...Object.values(meta.annotations ?? {}),
        ];
        return values.includes(workload.getName()) || meta.name.startsWith(`${workload.getName()}-`);
      });

      // Classify every workload pod (owned or fallback) so consumers know
      // which component/instance each pod plays.
      const workloadPods = [
        ...result.filter(resource => resource.kind === 'Pod'),
        ...workloadScoped,
      ];
      if (workloadPods.length) {
        const matches = await matchPodsToComponents(
          karta,
          workloadPods.map(resource => resource.item)
        ).catch(() => ({}) as Awaited<ReturnType<typeof matchPodsToComponents>>);
        for (const resource of workloadPods) {
          const match = matches[resource.item.metadata.name];
          if (match) {
            resource.component = match.component;
            resource.instance = match.instance;
          }
        }
        for (const resource of workloadScoped) {
          // Selector-matched fallback pods hang directly off the workload.
          if (resource.component) {
            resource.parentUid = workload.metadata.uid;
            result.push(resource);
          }
        }
      }

      for (const resource of result) {
        resource.parentUid ??= parents.get(resource.item.metadata.uid);
        resource.href = subResourceHref(resource);
      }
      result.sort(
        (a, b) =>
          a.kind.localeCompare(b.kind) || a.item.metadata.name.localeCompare(b.item.metadata.name)
      );

      if (!cancelled) {
        setResources(result);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [karta.metadata.uid, workload.metadata.uid]);

  return resources;
}
