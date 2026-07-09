// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { alpha, Box, Chip, darken, lighten, useTheme } from '@mui/material';

type PaletteKey = 'success' | 'info' | 'error' | 'warning' | 'secondary';

// One distinct color per normalized phase (v1alpha1.ResourceStatus values).
// Phases without an entry (and "Undefined") render grey.
const PHASE_PALETTE: Record<string, PaletteKey> = {
  Running: 'success',
  Completed: 'info',
  Failed: 'error',
  Degraded: 'warning',
  Initializing: 'secondary',
};

// PhaseChip tints the chip from the active theme palette. Text contrast is
// derived with lighten/darken from the main color rather than the palette's
// light/dark variants, which some themes define too close to the background.
function PhaseChip({ phase }: { phase: string }) {
  const theme = useTheme();
  const dark = theme.palette.mode === 'dark';
  const paletteKey = PHASE_PALETTE[phase];
  const main = paletteKey ? theme.palette[paletteKey].main : theme.palette.grey[dark ? 400 : 700];
  const text = dark ? lighten(main, 0.6) : darken(main, 0.35);

  return (
    <Chip
      size="small"
      label={phase}
      sx={{
        backgroundColor: alpha(main, dark ? 0.28 : 0.12),
        color: text,
        border: `1px solid ${alpha(main, 0.55)}`,
        fontWeight: 500,
      }}
    />
  );
}

export default function StatusPhaseChips({ phases }: { phases?: string[] }) {
  const shown = phases?.length ? phases : ['Undefined'];
  return (
    <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
      {shown.map(phase => (
        <PhaseChip key={phase} phase={phase} />
      ))}
    </Box>
  );
}
