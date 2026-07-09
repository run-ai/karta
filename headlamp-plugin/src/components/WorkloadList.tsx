// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Icon } from '@iconify/react';
import { Link, SectionBox, SimpleTable } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import type { KubeObject, KubeObjectClass } from '@kinvolk/headlamp-plugin/lib/k8s/cluster';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Typography,
} from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { useWorkloadTree } from '../api/tree';
import type { GroupVersionKind } from '../resources/karta';
import { KartaCR } from '../resources/karta';
import { useWorkloadClass } from '../resources/workload';
import { workloadTreeRoute } from '../routes';
import StatusPhaseChips from './StatusPhaseChips';

// Workload count per Karta name: undefined = still loading, null = the
// workload CRD is not applied in the cluster.
type Counts = Record<string, number | null | undefined>;

// One phase cell per row; each fetches the workload's tree from the operator
// to evaluate the Karta status mappings.
function WorkloadPhaseCell({ karta, workload }: { karta: KartaCR; workload: KubeObject }) {
  const { tree, error, loading } = useWorkloadTree(karta, workload);

  if (loading) {
    return (
      <Typography variant="caption" color="text.secondary">
        ...
      </Typography>
    );
  }
  if (error || !tree) {
    return (
      <Typography variant="caption" color="text.secondary" title={error?.message}>
        unavailable
      </Typography>
    );
  }
  return <StatusPhaseChips phases={tree.status?.phases} />;
}

// Rendered only once the workload class is resolved, so the useList hook is
// called unconditionally within this component.
function WorkloadsTable({
  karta,
  gvk,
  workloadClass,
  onCount,
}: {
  karta: KartaCR;
  gvk: GroupVersionKind;
  workloadClass: KubeObjectClass;
  onCount: (kartaName: string, count: number | null) => void;
}) {
  const [workloads, error] = workloadClass.useList();

  useEffect(() => {
    if (workloads) {
      onCount(karta.getName(), workloads.length);
    }
  }, [karta, workloads?.length, onCount]);

  if (error) {
    return <Alert severity="error">Failed to list workloads: {String(error)}</Alert>;
  }

  return (
    <SimpleTable
      columns={[
        {
          label: 'Name',
          getter: (workload: KubeObject) => (
            <Link
              routeName={workloadTreeRoute.name}
              params={{
                karta: karta.getName(),
                namespace: workload.getNamespace() ?? '-',
                name: workload.getName(),
              }}
            >
              {workload.getName()}
            </Link>
          ),
        },
        {
          label: 'Namespace',
          getter: (workload: KubeObject) => workload.getNamespace() ?? '',
        },
        { label: 'Kind', getter: () => gvk.kind },
        {
          label: 'Phase',
          getter: (workload: KubeObject) => <WorkloadPhaseCell karta={karta} workload={workload} />,
        },
        {
          label: 'Components',
          getter: () => karta.childComponentNames.join(', '),
        },
        {
          label: 'Age',
          getter: (workload: KubeObject) => workload.getAge(),
        },
      ]}
      data={workloads ?? []}
      emptyMessage="No workloads found for this Karta definition."
    />
  );
}

// One section per Karta definition. Each section owns its workload hooks, so
// the hook count stays fixed per rendered component even though the number of
// Kartas varies. Definitions whose workload CRD is not applied in the cluster
// render nothing and report a null count.
function KartaWorkloadsSection({
  karta,
  onCount,
}: {
  karta: KartaCR;
  onCount: (kartaName: string, count: number | null) => void;
}) {
  const gvk = karta.rootGVK;
  const [workloadClass, resolving] = useWorkloadClass(gvk);

  useEffect(() => {
    if (!gvk || (!workloadClass && !resolving)) {
      onCount(karta.getName(), null);
    }
  }, [karta, gvk, workloadClass, resolving, onCount]);

  if (!gvk || !workloadClass) {
    return null;
  }

  return (
    <SectionBox title={gvk.kind} headerProps={{ headerStyle: 'label' }}>
      <WorkloadsTable karta={karta} gvk={gvk} workloadClass={workloadClass} onCount={onCount} />
    </SectionBox>
  );
}

export default function WorkloadList() {
  const [kartas, error] = KartaCR.useList();
  const [counts, setCounts] = useState<Counts>({});

  const onCount = useCallback((kartaName: string, count: number | null) => {
    setCounts(prev => (prev[kartaName] === count ? prev : { ...prev, [kartaName]: count }));
  }, []);

  if (error) {
    return <Alert severity="error">Failed to list Karta definitions: {String(error)}</Alert>;
  }

  // Kinds still counting stay in the main area to avoid flicker; counted-empty
  // kinds collapse into the accordion below.
  const populated = (kartas ?? []).filter(karta => counts[karta.getName()] !== 0);
  const empty = (kartas ?? []).filter(karta => counts[karta.getName()] === 0);

  return (
    <SectionBox
      title="Karta Workloads"
      headerProps={{ headerStyle: 'subsection' }}
      backLink={false}
    >
      {kartas?.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No Karta definitions installed. Apply a Karta CR to describe a workload kind.
        </Typography>
      )}
      {populated.map(karta => (
        <KartaWorkloadsSection key={karta.getName()} karta={karta} onCount={onCount} />
      ))}
      {empty.length > 0 && (
        <Accordion disableGutters elevation={0} sx={{ mt: 2, '&:before': { display: 'none' } }}>
          <AccordionSummary expandIcon={<Icon icon="mdi:chevron-down" />}>
            <Typography variant="body2" color="text.secondary">
              Kinds without workloads ({empty.length})
            </Typography>
          </AccordionSummary>
          <AccordionDetails sx={{ pt: 0 }}>
            {empty.map(karta => (
              <KartaWorkloadsSection key={karta.getName()} karta={karta} onCount={onCount} />
            ))}
          </AccordionDetails>
        </Accordion>
      )}
    </SectionBox>
  );
}
