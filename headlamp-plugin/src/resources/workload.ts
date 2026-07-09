// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { K8s } from '@kinvolk/headlamp-plugin/lib';
import { makeCustomResourceClass } from '@kinvolk/headlamp-plugin/lib/Crd';
import type { KubeObjectClass } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { useMemo } from 'react';
import type { GroupVersionKind } from './karta';

// Karta targets CRD-backed workloads, so the workload class is resolved from
// the cluster's CustomResourceDefinitions: the CRD supplies the plural name
// and scope that cannot be derived from the GVK alone. Native kinds (e.g.
// Deployment) have no CRD and are not resolvable here.
export function useWorkloadClass(
  gvk: GroupVersionKind | null
): [KubeObjectClass | null, boolean, unknown] {
  const [crds, error] = K8s.ResourceClasses.CustomResourceDefinition.useList();

  const workloadClass = useMemo(() => {
    if (!gvk || !crds) {
      return null;
    }
    const crd = crds.find(
      item =>
        item.jsonData?.spec?.group === gvk.group && item.jsonData?.spec?.names?.kind === gvk.kind
    );
    if (!crd) {
      return null;
    }
    const names = crd.jsonData.spec.names;
    return makeCustomResourceClass({
      apiInfo: [{ group: gvk.group, version: gvk.version }],
      kind: gvk.kind,
      pluralName: names.plural,
      singularName: names.singular ?? gvk.kind.toLowerCase(),
      isNamespaced: crd.jsonData.spec.scope === 'Namespaced',
    });
  }, [gvk?.group, gvk?.version, gvk?.kind, crds]);

  const loading = crds === null;
  return [workloadClass, loading, error];
}
