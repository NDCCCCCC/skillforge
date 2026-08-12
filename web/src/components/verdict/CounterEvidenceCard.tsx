// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 — SC-6 / CMPL-06 / D5-13.
//
// CounterEvidenceCard — the visible trust-moat marker. Renders inline
// AFTER VerdictCard as a sibling (NOT inside VerdictCard) so the visual
// hierarchy matches the semantic boundary: the verdict reads ONLY 权威
// (CMPL-04 / D-09 trust floor), and counter-evidence is ANNOTATION from
// the 历史 tier that NEVER entered grounding.
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

import { AlertTriangle } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { ConflictChunk } from '../../types/wire-types';
import type { Tier } from '../../types/verdict';

/** Props — conflictChunks only (D5-13: never receives verdict.citations). */
export interface CounterEvidenceCardProps {
  conflictChunks?: ConflictChunk[];
}

/** The hardcoded disclaimer sentence (UI-SPEC §SC-6 — never customized). */
const DISCLAIMER =
  '以下内容来自「历史」知识层，仅供人工参考，不参与本次结论的生成与校验。';

export default function CounterEvidenceCard({ conflictChunks }: CounterEvidenceCardProps) {
  const setCenterView = useAppStore((s) => s.setCenterView);
  const rows = conflictChunks ?? [];

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

      {rows.length === 0 ? (
        <p
          data-testid="counter-evidence-empty"
          className="mt-3 text-xs text-text-dim"
        >
          未检索到与本次结论冲突的历史片段。
        </p>
      ) : (
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
      )}
    </div>
  );
}
