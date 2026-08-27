// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const { getKartaEngine } = vi.hoisted(() => ({ getKartaEngine: vi.fn() }));
vi.mock('../../lib/karta/karta', () => ({ getKartaEngine }));

import { useKartaEngine } from './useKartaEngine';

describe('useKartaEngine', () => {
  it('starts loading, then exposes the resolved engine', async () => {
    const engine = { buildTree: vi.fn(), attributePods: vi.fn(), evaluatePhases: vi.fn(), listCatalog: vi.fn() };
    getKartaEngine.mockResolvedValue(engine);

    const { result } = renderHook(() => useKartaEngine());

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.engine).toBe(engine));
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('exposes the error when the engine fails to load', async () => {
    getKartaEngine.mockRejectedValue(new Error('wasm load failed'));

    const { result } = renderHook(() => useKartaEngine());

    await waitFor(() => expect(result.current.error).not.toBeNull());
    expect(result.current.error?.message).toBe('wasm load failed');
    expect(result.current.engine).toBeNull();
    expect(result.current.loading).toBe(false);
  });
});
