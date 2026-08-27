// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { useEffect, useState } from 'react';
import { Definition, KartaCR, mergeDefinitions } from '../../lib/karta/definitions';
import { Karta } from '../../lib/karta/karta.types';
import { listCatalog } from '../../lib/karta/kartaUtil';

export interface UseKartaDefinitionsResult {
  definitions: Definition[];
  installed: boolean;
  crdMissing: boolean;
  loading: boolean;
  error: Error | null;
}

// useKartaDefinitions merges cluster kartas.run.ai CRs (via KartaCR.useList(),
// which watches — so this automatically re-merges on every CR addition or
// deletion) with the WASM engine's embedded catalog. When Karta isn't
// installed in the cluster (CRD missing) it falls back to catalog-only
// definitions instead of blocking.
export function useKartaDefinitions(): UseKartaDefinitionsResult {
  const [catalog, setCatalog] = useState<Karta[]>([]);
  const [catalogError, setCatalogError] = useState<Error | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [clusterKartas, clusterError] = KartaCR.useList();

  useEffect(() => {
    let cancelled = false;

    listCatalog()
      .then(list => {
        if (!cancelled) {
          setCatalog(list);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setCatalogError(err instanceof Error ? err : new Error(String(err)));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setCatalogLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const crdMissing = clusterError?.status === 404;
  const installed = clusterError === null;
  const cluster = installed ? (clusterKartas ?? []).map(item => item.jsonData as Karta) : [];

  return {
    definitions: mergeDefinitions(catalog, cluster),
    installed,
    crdMissing,
    loading: catalogLoading,
    error: catalogError ?? (clusterError && !crdMissing ? clusterError : null),
  };
}
