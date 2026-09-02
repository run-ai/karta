// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { registerRoute, registerSidebarEntry } from '@kinvolk/headlamp-plugin/lib';
import { useEffect, useState } from 'react';
import { getKartaEngine } from './lib/engine';

registerSidebarEntry({
  parent: null,
  name: 'karta',
  label: 'Karta',
  url: '/karta/workloads',
  icon: 'mdi:graph-outline',
});

function WorkloadsPlaceholder() {
  const [engineVersion, setEngineVersion] = useState<string | null>(null);
  const [engineError, setEngineError] = useState<string | null>(null);

  useEffect(() => {
    getKartaEngine()
      .then(engine => setEngineVersion(engine.version()))
      .catch(err => setEngineError(err instanceof Error ? err.message : String(err)));
  }, []);

  return (
    <div>
      <p>Karta workloads — coming soon.</p>
      <p>
        WASM engine:{' '}
        {engineError ? `unavailable (${engineError})` : (engineVersion ?? 'loading…')}
      </p>
    </div>
  );
}

registerRoute({
  path: '/karta/workloads',
  sidebar: 'karta',
  name: 'karta-workloads',
  exact: true,
  component: WorkloadsPlaceholder,
});
