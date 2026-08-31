// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { registerRoute, registerSidebarEntry } from '@kinvolk/headlamp-plugin/lib';
import { WorkloadsTable } from './components';
import { useWorkloadRows } from './hooks';
import { getKartaEngine } from './lib/karta';

// Kick off the WASM engine load (fetch + instantiate karta.wasm, ~19MB) as
// soon as the plugin's module loads — i.e. when Headlamp itself starts —
// instead of waiting for the user to navigate to the Workloads page.
// getKartaEngine() caches its promise, so useKartaEngine() (called from
// WorkloadsPage) reuses this same in-flight/resolved load rather than
// starting a second one. Errors are swallowed here only to avoid an
// unhandled-rejection log; getKartaEngine() itself resets its cache on
// failure, so useKartaEngine() still retries and surfaces the real error
// when the page mounts.
getKartaEngine().catch(() => {});

registerSidebarEntry({
  parent: null,
  name: 'karta',
  label: 'Karta',
  icon: 'mdi:graph-outline',
});

registerSidebarEntry({
  parent: 'karta',
  name: 'karta-workloads',
  label: 'Workloads',
  url: '/karta/workloads',
});

function WorkloadsPage() {
  const { rows, loading, error, fetchers } = useWorkloadRows();
  // A single kind failing to list (e.g. its CRD isn't installed, or a
  // transient watch error) must not blank rows other kinds already loaded
  // successfully — Headlamp's Table replaces all rows with errorMessage
  // whenever it's set, so only surface it when there's nothing else to show.
  const errorMessage = rows && rows.length > 0 ? null : error?.message;
  return (
    <>
      {fetchers}
      <WorkloadsTable rows={rows} loading={loading} errorMessage={errorMessage} />
    </>
  );
}

registerRoute({
  path: '/karta/workloads',
  sidebar: 'karta-workloads',
  name: 'karta-workloads',
  exact: true,
  component: WorkloadsPage,
});
