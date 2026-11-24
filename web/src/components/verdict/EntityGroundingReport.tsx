// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 57-03 (ONT-03) -- the entity grounding report surface
// (57-UI-SPEC section 2.2).
//
// HONESTY CONTRACT (the whole point of this component):
//   - `entities` ABSENT means entity grounding did not run (no KG wired).
//     That is NOT a failure, and nothing is rendered -- no "无", no
//     "0", no grey placeholder. Silence is the correct rendering of
//     "this check did not happen".
//   - `unverified` NON-EMPTY is a real risk signal: the answer named
//     entities the authorized corpus does not know, and the BE has
//     already forced review_required. The copy says exactly that.
//   - READ-ONLY: zero buttons, zero handlers (55-02 Honesty posture).
import { ShieldAlert, ShieldCheck } from 'lucide-react';
import type { EntityGrounding } from '../../types/verdict';

export interface EntityGroundingReportProps {
  entities?: EntityGrounding;
}

export default function EntityGroundingReport({ entities }: EntityGroundingReportProps) {
  // Did not run -> say nothing at all (see the honesty contract above).
  if (!entities) return null;

  const verified = Array.isArray(entities.verified) ? entities.verified : [];
  const unverified = Array.isArray(entities.unverified) ? entities.unverified : [];

  if (unverified.length > 0) {
    return (
      <section
        data-testid="entity-grounding-report"
        data-verdict="unverified"
        className="mt-3 rounded-md border border-status-non-compliant/40 bg-status-non-compliant/5 px-3 py-2"
      >
        <h3 className="flex items-center gap-1.5 text-xs font-semibold text-status-non-compliant">
          <ShieldAlert className="h-3.5 w-3.5" />
          实体校验未通过
        </h3>
        <ul className="mt-1 flex flex-col gap-0.5">
          {unverified.map((name) => (
            <li
              key={name}
              data-testid="entity-unverified-item"
              className="text-xs text-status-non-compliant"
            >
              {name}
            </li>
          ))}
        </ul>
        <p className="mt-1 text-xs text-text-muted">
          这些实体在本 Skill 可读语料中不存在，回答已降级为需复核
        </p>
      </section>
    );
  }

  if (verified.length > 0) {
    return (
      <details
        data-testid="entity-grounding-report"
        data-verdict="verified"
        className="mt-3 rounded-md border border-status-compliant/40 bg-status-compliant/5 px-3 py-2"
      >
        <summary className="flex cursor-pointer items-center gap-1.5 text-xs font-semibold text-status-compliant">
          <ShieldCheck className="h-3.5 w-3.5" />
          实体校验通过（{verified.length} 个）
        </summary>
        <ul className="mt-1 flex flex-col gap-0.5">
          {verified.map((name) => (
            <li key={name} data-testid="entity-verified-item" className="text-xs text-text-muted">
              {name}
            </li>
          ))}
        </ul>
      </details>
    );
  }

  return (
    <div
      data-testid="entity-grounding-report"
      data-verdict="empty"
      className="mt-3 text-xs text-text-muted"
    >
      实体校验：未识别到实体名称
    </div>
  );
}
