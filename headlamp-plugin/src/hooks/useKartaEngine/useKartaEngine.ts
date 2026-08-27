// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { useEffect, useState } from 'react';
import { getKartaEngine, KartaEngine } from '../../lib/karta/karta';

export interface UseKartaEngineResult {
  engine: KartaEngine | null;
  error: Error | null;
  loading: boolean;
}

// useKartaEngine loads the WASM engine once and exposes it to components.
// getKartaEngine() itself caches the load, so re-mounting this hook
// elsewhere in the tree does not re-instantiate the module.
export function useKartaEngine(): UseKartaEngineResult {
  const [engine, setEngine] = useState<KartaEngine | null>(null);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;

    getKartaEngine()
      .then(loaded => {
        if (!cancelled) {
          setEngine(loaded);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return { engine, error, loading: !engine && !error };
}
