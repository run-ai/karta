// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { render, waitFor } from '@testing-library/react';
import { useEffect } from 'react';
import { describe, expect, it, vi } from 'vitest';

const { useKartaEngine, useKartaDefinitions } = vi.hoisted(() => ({
  useKartaEngine: vi.fn(),
  useKartaDefinitions: vi.fn(),
}));
vi.mock('../useKartaEngine', () => ({ useKartaEngine }));
vi.mock('../useKartaDefinitions', () => ({ useKartaDefinitions }));

vi.mock('./KindFetcher', () => ({
  KindFetcher: ({ definition, onRows, onError }: any) => {
    useEffect(() => {
      const name = definition.karta.metadata.name;
      if (name === 'broken') {
        onError(name, new Error(`${name} failed to list`));
      } else if (name !== 'pending') {
        onRows(name, [{ id: name, name }]);
      }
      // 'pending' never calls onRows/onError, simulating a kind whose
      // useList() hasn't resolved its first page yet.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return null;
  },
}));

import type { Definition } from '../../lib/karta/definitions';
import type { UseWorkloadRowsResult } from './useWorkloadRows';
import { useWorkloadRows } from './useWorkloadRows';

function definition(name: string): Definition {
  return {
    karta: {
      apiVersion: 'run.ai/v1alpha1',
      kind: 'Karta',
      metadata: { name },
      spec: { structureDefinition: { rootComponent: {} } },
    },
    origin: 'catalog',
  };
}

function Harness({ onResult }: { onResult: (result: UseWorkloadRowsResult) => void }) {
  const result = useWorkloadRows();
  onResult(result);
  return <>{result.fetchers}</>;
}

function renderHarness() {
  let latest: UseWorkloadRowsResult | undefined;
  const { rerender } = render(<Harness onResult={r => (latest = r)} />);
  return {
    get current() {
      return latest!;
    },
    rerender: () => rerender(<Harness onResult={r => (latest = r)} />),
  };
}

describe('useWorkloadRows', () => {
  it('reports loading while the engine or definitions are still loading', () => {
    useKartaEngine.mockReturnValue({ engine: null, loading: true, error: null });
    useKartaDefinitions.mockReturnValue({ definitions: [], loading: false, error: null, installed: true, crdMissing: false });

    const harness = renderHarness();

    expect(harness.current.loading).toBe(true);
    expect(harness.current.rows).toBeNull();
  });

  it('aggregates rows from each definition once fetchers resolve', async () => {
    useKartaEngine.mockReturnValue({ engine: {}, loading: false, error: null });
    useKartaDefinitions.mockReturnValue({
      definitions: [definition('deployment'), definition('pytorchjob')],
      loading: false,
      error: null,
      installed: true,
      crdMissing: false,
    });

    const harness = renderHarness();

    await waitFor(() => expect(harness.current.rows).toHaveLength(2));
    expect(harness.current.rows).toEqual(
      expect.arrayContaining([
        { id: 'deployment', name: 'deployment' },
        { id: 'pytorchjob', name: 'pytorchjob' },
      ])
    );
    expect(harness.current.loading).toBe(false);
    expect(harness.current.error).toBeNull();
  });

  it('surfaces the engine error and never blocks on a single kind failing', async () => {
    useKartaEngine.mockReturnValue({ engine: null, loading: false, error: new Error('wasm load failed') });
    useKartaDefinitions.mockReturnValue({
      definitions: [definition('broken')],
      loading: false,
      error: null,
      installed: true,
      crdMissing: false,
    });

    const harness = renderHarness();

    await waitFor(() => expect(harness.current.error?.message).toBe('wasm load failed'));
  });

  it('stays loading until every definition has reported rows or an error', async () => {
    useKartaEngine.mockReturnValue({ engine: {}, loading: false, error: null });
    useKartaDefinitions.mockReturnValue({
      definitions: [definition('deployment'), definition('pending')],
      loading: false,
      error: null,
      installed: true,
      crdMissing: false,
    });

    const harness = renderHarness();

    // 'deployment' resolves immediately (its effect already ran by the time
    // render() returns) but 'pending' never does — the table must not flip
    // to its empty/loaded state on 'deployment' alone.
    expect(harness.current.loading).toBe(true);
    expect(harness.current.rows).toBeNull();
  });

  it('surfaces a per-kind list error when the engine itself is fine', async () => {
    useKartaEngine.mockReturnValue({ engine: {}, loading: false, error: null });
    useKartaDefinitions.mockReturnValue({
      definitions: [definition('broken')],
      loading: false,
      error: null,
      installed: true,
      crdMissing: false,
    });

    const harness = renderHarness();

    await waitFor(() => expect(harness.current.error?.message).toBe('broken failed to list'));
  });
});
