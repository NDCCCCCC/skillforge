// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 + Plan 260815-cos Task 3 — SC-6 / CMPL-06 / D5-13.
//
// CounterEvidenceCard — the visible trust-moat marker. Renders inline
// AFTER VerdictCard as a sibling (NOT inside VerdictCard) so the visual
// hierarchy matches the semantic boundary: the verdict grounds ONLY on
// authority-role knowledge bases (CMPL-04 / D-09 trust floor — the single
// citable role in the Phase 26 registry), and counter-evidence is
// ANNOTATION from historical-role bases that NEVER entered grounding.
//
// The card's left border + the 历史 tier badge use --color-annotation
// (teal) — the ONLY teal surface in the app (05-05 SUMMARY: GraphTree
// deliberately does NOT use teal for tier text so this line stays bright).
//
// The DISCLAIMER SENTENCE is a hardcoded JSX literal — UI-SPEC §SC-6
// requires it on every render. T-5-27 mitigation: no prop allows
// customization; the test asserts the exact string is present verbatim.
//
// Threats mitigated:
//   - T-5-27: disclaimer is a JSX literal, not customizable. Test asserts verbatim.
//   - T-5-28: no dangerouslySetInnerHTML anywhere in this file.
//   - D5-13: this component receives ONLY conflictChunks (NOT verdict.citations) —
//            the trust moat is mirrored on the FE side as a prop-isolation rule.
//
// Plan 260815-cos / locked CONTEXT.md decision E1: when there are zero
// conflict chunks the component returns null — the card vanishes entirely
// (no disclaimer header, no empty-state copy, no surrounding chrome).
// The trust floor (default-deny on no annotations) means the absence of
// counter-evidence is the boring case; surfacing the disclaimer on every
// verdict overstates the annotation's significance. The non-empty render
// keeps the disclaimer + list (annotation rows).

import { AlertTriangle } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { ConflictChunk } from '../../types/wire-types';
import type { Tier } from '../../types/verdict';

/** Props — conflictChunks only (D5-13: never receives verdict.citations). */
export interface CounterEvidenceCardProps {
  conflictChunks?: ConflictChunk[];
}

/** The hardcoded disclaimer sentence (UI-SPEC §SC-6 — never customized).
 * WHITELIST NOTE (26-10 CN-tier gate, review 2026-10): 「历史」 here is the
 * historical ROLE in locked user-facing copy — the SC-6 disclaimer is a
 * test-pinned JSX literal (T-5-27), not a tier value, comparison, or
 * default. Tier *identifiers* in this file are registry slugs. */
const DISCLAIMER =
  '以下内容来自「历史」知识层，仅供人工参考，不参与本次结论的生成与校验。';

export default function CounterEvidenceCard({ conflictChunks }: CounterEvidenceCardProps) {
  // Plan 260815-cos / decision E1: zero rows → return null. The empty-
  // state copy is GONE — the disclaimer + header chrome only appear
  // when there ARE conflict chunks to annotate. This prevents the
  // disclaimer from being shown on every verdict (where the boring
  // case = no annotation = no card).
  const rows = conflictChunks ?? [];
  if (rows.length === 0) return null;

  const setCenterView = useAppStore((s) => s.setCenterView);

  return (
    <div
      data-testid="counter-evidence-card"
      data-conflict-count={rows.length}
      className={cn(
        'my-2 rounded-lg border-0 border-l-4 bg-surface pl-4 pr-5 py-3 text-text',
        'border-l-annotation',
      )}
    >
      <div className="flex items-center gap-2">
        <AlertTriangle size={16} className="text-annotation" aria-hidden="true" />
        <span className="text-base font-semibold text-annotation">历史层反证</span>
      </div>

      {/* DISCLAIMER — hardcoded JSX literal. T-5-27: test asserts this string verbatim. */}
      <p
        data-testid="counter-evidence-disclaimer"
        className="mt-2 text-base text-text-muted"
      >
        {DISCLAIMER}
      </p>

      <ul className="mt-3 space-y-1.5" data-testid="counter-evidence-rows">
        {rows.map((row) => (
          <li key={row.chunkId}>
            <button
              type="button"
              data-testid="counter-evidence-row"
              data-chunk-id={row.chunkId}
              onClick={() =>
                setCenterView({
                  kind: 'chunk-source',
                  chunkId: row.chunkId,
                  tier: row.tier as Tier,
                  version: null,
                  effectiveDate:
                    row.effectiveFrom && row.effectiveFrom > 0
                      ? String(row.effectiveFrom)
                      : null,
                  effectiveTo:
                    row.effectiveTo && row.effectiveTo > 0 ? row.effectiveTo : null,
                })
              }
              className="flex w-full items-start gap-2 rounded-md border border-border bg-surface-2 px-2 py-1.5 text-left hover:bg-surface-3"
            >
              {/* 历史 tier badge in teal — the SCOPED §Color exception. */}
              <span
                data-testid="counter-evidence-tier-badge"
                data-tier={row.tier}
                className="rounded-full bg-annotation-dim px-1.5 py-0.5 text-[11px] font-semibold text-annotation"
              >
                {row.tier}
              </span>
              <span className="font-mono text-xs text-text-muted">{row.chunkId}</span>
              <span className="flex-1 truncate text-xs">{row.snippet}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
