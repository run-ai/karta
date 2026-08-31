// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Heuristic English pluralization for a Kubernetes Kind, used to derive the
// resource's plural name (needed by makeCustomResourceClass) when a Karta
// definition only carries the Kind (e.g. "PyTorchJob"), not the plural
// ("pytorchjobs"). Covers regular pluralization; kinds with irregular
// plurals (rare in practice) need an explicit override — none exist in the
// embedded catalog today.
export function pluralize(kind: string): string {
  const lower = kind.toLowerCase();
  if (/(s|x|z|ch|sh)$/.test(lower)) {
    return `${lower}es`;
  }
  if (/[^aeiou]y$/.test(lower)) {
    return `${lower.slice(0, -1)}ies`;
  }
  return `${lower}s`;
}
