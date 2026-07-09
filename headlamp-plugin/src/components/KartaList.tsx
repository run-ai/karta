// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Link, SectionBox, SimpleTable } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import { Alert, Chip, Tooltip } from '@mui/material';
import { KartaCR } from '../resources/karta';

// The CRD's metadata.name, used by Headlamp's built-in custom resource route.
const KARTA_CRD_NAME = 'kartas.run.ai';

// ReadyChip surfaces the operator's Ready condition, which requires the spec
// to validate and the target CRD to exist in the cluster.
function ReadyChip({ karta }: { karta: KartaCR }) {
  const condition = karta.readyCondition;
  if (!condition) {
    return (
      <Tooltip title="No status reported; is the Karta operator running?">
        <Chip size="small" label="Unknown" />
      </Tooltip>
    );
  }
  if (condition.status === 'True') {
    return <Chip size="small" color="success" label="Ready" />;
  }
  return (
    <Tooltip title={condition.message ?? condition.reason ?? ''}>
      <Chip size="small" color="error" label={condition.reason ?? 'Not Ready'} />
    </Tooltip>
  );
}

export default function KartaList() {
  const [kartas, error] = KartaCR.useList();

  if (error) {
    return <Alert severity="error">Failed to list Karta definitions: {String(error)}</Alert>;
  }

  return (
    <SectionBox
      title="Karta Definitions"
      headerProps={{ headerStyle: 'subsection' }}
      backLink={false}
    >
      <SimpleTable
        columns={[
          {
            label: 'Name',
            getter: (karta: KartaCR) => (
              <Link
                routeName="customresource"
                params={{ crd: KARTA_CRD_NAME, namespace: '-', crName: karta.getName() }}
              >
                {karta.getName()}
              </Link>
            ),
          },
          {
            label: 'Workload Kind',
            getter: (karta: KartaCR) => {
              const gvk = karta.rootGVK;
              return gvk ? `${gvk.group ? `${gvk.group}/` : ''}${gvk.version} ${gvk.kind}` : '';
            },
          },
          {
            label: 'Components',
            getter: (karta: KartaCR) => karta.childComponentNames.join(', '),
          },
          { label: 'Ready', getter: (karta: KartaCR) => <ReadyChip karta={karta} /> },
          { label: 'Age', getter: (karta: KartaCR) => karta.getAge() },
        ]}
        data={kartas ?? []}
        emptyMessage="No Karta definitions installed."
      />
    </SectionBox>
  );
}
