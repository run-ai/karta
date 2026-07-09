// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { describe, expect, it } from 'vitest';
import type { SubResource } from '../api/resources';
import { hierarchyRows } from './WorkloadSubResources';

function resource(kind: string, name: string, uid: string, parentUid?: string): SubResource {
  return { kind, item: { metadata: { uid, name } }, parentUid };
}

describe('hierarchyRows', () => {
  it('places children below their parent with increasing depth', () => {
    const rows = hierarchyRows(
      [
        resource('Pod', 'job-a-pod-0', 'pod-a0', 'job-a'),
        resource('Job', 'job-a', 'job-a', 'wl'),
        resource('Job', 'job-b', 'job-b', 'wl'),
        resource('Pod', 'job-b-pod-0', 'pod-b0', 'job-b'),
      ],
      'wl'
    );

    expect(rows.map(row => [row.resource.item.metadata.name, row.depth])).toEqual([
      ['job-a', 0],
      ['job-a-pod-0', 1],
      ['job-b', 0],
      ['job-b-pod-0', 1],
    ]);
  });

  it('treats resources without a known parent as direct children', () => {
    const rows = hierarchyRows([resource('Pod', 'orphan', 'pod-x')], 'wl');
    expect(rows).toHaveLength(1);
    expect(rows[0].depth).toBe(0);
  });
});
