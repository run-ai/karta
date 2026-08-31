// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { CommonComponents } from '@kinvolk/headlamp-plugin/lib';
import type { TableColumn } from '@kinvolk/headlamp-plugin/lib/CommonComponents';
import { ReactNode, useMemo } from 'react';

const { Table, Loader } = CommonComponents;

export interface DataTableProps<RowItem extends Record<string, any>> {
  // Identifies this table for URL-reflected page/sort state — must be
  // unique among tables rendered on the same page.
  id: string;
  data: RowItem[] | null;
  columns: TableColumn<RowItem>[];
  initialSortColumnId?: string;
  initialSortDesc?: boolean;
  // Column ids to start hidden — still toggleable via the table's built-in
  // column picker, unlike Table's own hideColumns (permanently hidden).
  hiddenColumnIds?: string[];
  loading?: boolean;
  errorMessage?: string | null;
  emptyMessage?: ReactNode;
}

// DataTable wraps Headlamp's Table (CommonComponents) with the sorting/
// hideable-column/URL-reflection setup every page's table needs, so pages
// don't each re-derive that boilerplate. Not ResourceTable/ResourceListView:
// those require rows to extend KubeObject, which Karta-computed rows aren't.
export function DataTable<RowItem extends Record<string, any>>({
  id,
  data,
  columns,
  initialSortColumnId,
  initialSortDesc = true,
  hiddenColumnIds,
  loading,
  errorMessage,
  emptyMessage,
}: DataTableProps<RowItem>) {
  const initialState = useMemo(() => {
    const state: { sorting?: { id: string; desc: boolean }[]; columnVisibility?: Record<string, boolean> } = {};
    if (initialSortColumnId) {
      state.sorting = [{ id: initialSortColumnId, desc: initialSortDesc }];
    }
    if (hiddenColumnIds?.length) {
      state.columnVisibility = Object.fromEntries(hiddenColumnIds.map(colId => [colId, false]));
    }
    return state;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialSortColumnId, initialSortDesc, hiddenColumnIds?.join(',')]);

  // Table's own `loading` prop replaces its entire render — toolbar, search
  // box, column headers, everything — with a bare spinner (see Table.js's
  // `if (loading) return <Loader .../>` branch, which runs before the
  // header/toolbar are rendered at all). That's fine for a first paint but
  // wrong once the page chrome is already up: the user loses the toolbar
  // and column headers for as long as loading lasts. So `loading` is never
  // forwarded to Table itself — instead it's folded into `emptyMessage`,
  // which Table renders inside its normal (chrome-intact) empty-state path.
  const resolvedEmptyMessage = loading ? <Loader title="Loading table data" /> : emptyMessage;

  return (
    <Table
      data={data ?? []}
      columns={columns}
      errorMessage={errorMessage ?? undefined}
      emptyMessage={resolvedEmptyMessage}
      initialState={initialState}
      reflectInURL={id}
    />
  );
}
