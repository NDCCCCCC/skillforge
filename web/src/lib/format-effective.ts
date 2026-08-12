// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-8 (CMPL-08) — small date-formatting helper for the
// effective_from / effective_to rows in ChunkSourceView + the
// FilteredOutNotice banner. Converts epoch-seconds to a YYYY-MM-DD
// UTC string for stable rendering (the BE carries dates in epoch-
// seconds to keep the wire compact; the FE presents them in local-
// neutral UTC form for cross-machine reproducibility in tests).

/**
 * Render an epoch-seconds value as YYYY-MM-DD (UTC). Returns the
 * literal 未知 when the input is null, zero, or negative — matching
 * the 04-UI-SPEC empty-state copy.
 */
export function formatEffectiveDate(epochSec: number | null | undefined): string {
  if (epochSec === null || epochSec === undefined || epochSec <= 0) {
    return '未知';
  }
  const d = new Date(epochSec * 1000);
  // Guard against an Invalid Date (defensive — the BE rejects negative
  // inputs at the indexer layer; this is belt-and-suspenders).
  if (Number.isNaN(d.getTime())) {
    return '未知';
  }
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}