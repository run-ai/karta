// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Router } from '@kinvolk/headlamp-plugin/lib';
import {
  Loader,
  NameValueTable,
  SectionBox,
} from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import type { KubeObject, KubeObjectClass } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import { Alert, Link as MuiLink } from '@mui/material';
import { useState } from 'react';
import { Link as RouterLink, useParams } from 'react-router-dom';
import { usePodUsage } from '../api/metrics';
import {
  addTotals,
  aggregateTreeRequests,
  containerRequests,
  EMPTY_TOTALS,
  formatCpu,
  formatMemory,
  formatTotals,
} from '../api/quantities';
import { useWorkloadResources } from '../api/resources';
import { useWorkloadTree } from '../api/tree';
import { KartaCR } from '../resources/karta';
import { useWorkloadClass } from '../resources/workload';
import { workloadListRoute } from '../routes';
import type { PodInfo } from './flowLayout';
import WorkloadSubResources from './WorkloadSubResources';
import WorkloadTreePanel from './WorkloadTreePanel';

interface TreeRouteParams {
  karta: string;
  namespace: string;
  name: string;
}

// Rendered only once the workload is loaded, so the resources hook runs
// unconditionally within this component.
function WorkloadTreeSections({
  karta,
  workload,
  gvk,
  workloadClass,
}: {
  karta: KartaCR;
  workload: KubeObject;
  gvk: NonNullable<KartaCR['rootGVK']>;
  workloadClass: KubeObjectClass;
}) {
  const resources = useWorkloadResources(karta, workload);
  const { tree } = useWorkloadTree(karta, workload);

  const pods: PodInfo[] = (resources ?? [])
    .filter(resource => resource.kind === 'Pod' && resource.component)
    .map(resource => ({
      name: resource.item.metadata.name,
      phase: resource.item.status?.phase,
      nodeName: resource.item.spec?.nodeName,
      component: resource.component!,
      instance: resource.instance,
      link: resource.href,
    }));

  // Requested: desired resources from the workload spec (replicas x container
  // requests). Actual: live cpu/mem usage from the metrics API plus the GPUs
  // allocated by currently running pods (GPU usage is not in metrics-server).
  const workloadPods = (resources ?? []).filter(resource => resource.kind === 'Pod');
  const usage = usePodUsage(
    workload.getNamespace(),
    workloadPods.map(resource => resource.item.metadata.name)
  );
  const requested = tree ? aggregateTreeRequests(tree) : null;
  const gpusAllocated = workloadPods
    .filter(resource => resource.item.status?.phase === 'Running')
    .flatMap(resource => resource.item.spec?.containers ?? [])
    .reduce((sum, container) => addTotals(sum, containerRequests(container)), EMPTY_TOTALS).gpus;

  let actualValue = 'loading...';
  if (usage) {
    const cpuMem = usage.available
      ? `cpu ${formatCpu(usage.cpuMillis)} · mem ${formatMemory(usage.memoryBytes)}`
      : 'cpu - · mem - (metrics API unavailable)';
    actualValue = `${cpuMem} · gpu ${gpusAllocated} allocated`;
  }

  const rootLink = Router.createRouteURL('customresource', {
    crd: `${workloadClass.apiName}.${gvk.group}`,
    namespace: workload.getNamespace() ?? '-',
    crName: workload.getName(),
  });

  return (
    <>
      <SectionBox title="Workload" headerProps={{ headerStyle: 'label' }}>
        <NameValueTable
          rows={[
            {
              name: 'Name',
              value: (
                <MuiLink component={RouterLink} to={rootLink}>
                  {workload.getName()}
                </MuiLink>
              ),
            },
            { name: 'Namespace', value: workload.getNamespace() ?? 'cluster-scoped' },
            {
              name: 'Kind',
              value: `${gvk.group ? `${gvk.group}/` : ''}${gvk.version} ${gvk.kind}`,
            },
            { name: 'Karta', value: karta.getName() },
            {
              name: 'Requested Resources',
              value: requested ? formatTotals(requested) : 'loading...',
            },
            { name: 'Actual Usage', value: actualValue },
          ]}
        />
      </SectionBox>
      <SectionBox title="Workload Tree" headerProps={{ headerStyle: 'label' }}>
        <WorkloadTreePanel karta={karta} workload={workload} pods={pods} rootLink={rootLink} />
      </SectionBox>
      <SectionBox title="Resources" headerProps={{ headerStyle: 'label' }}>
        <WorkloadSubResources resources={resources} workload={workload} />
      </SectionBox>
    </>
  );
}

function WorkloadTreeInner({
  karta,
  workloadClass,
  params,
}: {
  karta: KartaCR;
  workloadClass: KubeObjectClass;
  params: TreeRouteParams;
}) {
  const gvk = karta.rootGVK;
  const [workload, setWorkload] = useState<KubeObject | null>(null);
  const namespace = params.namespace === '-' ? undefined : params.namespace;

  workloadClass.useApiGet(setWorkload, params.name, namespace);

  if (!workload || !gvk) {
    return <Loader title="Loading workload" />;
  }

  return (
    <WorkloadTreeSections
      karta={karta}
      workload={workload}
      gvk={gvk}
      workloadClass={workloadClass}
    />
  );
}

function WorkloadTreeContent({ karta, params }: { karta: KartaCR; params: TreeRouteParams }) {
  const gvk = karta.rootGVK;
  const [workloadClass, resolving] = useWorkloadClass(gvk);

  if (!workloadClass) {
    if (resolving) {
      return <Loader title="Resolving workload kind" />;
    }
    return (
      <Alert severity="warning">
        No CustomResourceDefinition found for the workload kind of Karta "{karta.getName()}".
      </Alert>
    );
  }

  return <WorkloadTreeInner karta={karta} workloadClass={workloadClass} params={params} />;
}

export default function WorkloadTreeView() {
  const params = useParams<TreeRouteParams>();
  const [karta, setKarta] = useState<KartaCR | null>(null);

  KartaCR.useApiGet(item => setKarta(item as KartaCR), params.karta);

  return (
    <SectionBox
      title={`Workload Tree: ${params.name}`}
      headerProps={{ headerStyle: 'subsection' }}
      backLink={Router.createRouteURL(workloadListRoute.name)}
    >
      {karta ? (
        <WorkloadTreeContent karta={karta} params={params} />
      ) : (
        <Loader title="Loading Karta definition" />
      )}
    </SectionBox>
  );
}
