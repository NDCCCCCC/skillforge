// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// RoutingTracePart — Plan 05-03 / SC-1 (SYSK-05) per-turn routing trace.
//
// Renders the routing decision chunk that the backend emits on EVERY
// routed turn (D5-04). The trace shows:
//   - One row per Skill candidate: `{skillName} · cos {score}`
//     - Selected row: rendered in `--color-primary` (accent reserved surface #15)
//     - Others: rendered in `--color-text-muted`
//   - When `ambiguous === true`: reuses the SkillPicker ambiguity mode
//     below the score rows with `使用此 Skill` / `使用默认 Skill` CTAs.
//
// Defense (per UI-SPEC §Copywriting Contract):
//   - Empty / missing skillScores on a routed chunk → renders `路由信息缺失。`
//   - Non-routed decisions (`retrieve` / `answer` / `refuse` / `forced_stop`)
//     are NOT routed here — the MessageBubble dispatcher routes
//     `data-decision` with decision='route' | 'counter_evidence' to this
//     component and lets other decision values silently drop (forward-
//     compat with Phase 3 inspector data).
//
// Threat model: T-5-13 — score is rendered via toFixed(2) + string
// interpolation only; NEVER via dangerouslySetInnerHTML. The candidate
// name comes from server-rendered `skillName` strings and is treated
// as text (not HTML).
import type { UIMessage } from 'ai';
import SkillPicker from './SkillPicker';
import type { DecisionPayload, SkillScore } from '../../../types/wire-types';

// AI SDK v7's UIMessage generic UIDataTypes defaults do not include
// the custom data-* payloads we author. Cast through `unknown` to the
// structural shape the renderer actually reads (Pitfall 2: payload
// lives on .data, NOT .policy).
type RoutingPartType = {
  type: 'data-decision';
  id?: string;
  data: DecisionPayload;
};

export interface RoutingTracePartProps {
  part: RoutingPartType | (UIMessage['parts'][number] & Record<string, unknown>);
  /** Plan 05-03 — invoked when the user picks a candidate in the ambiguity picker. */
  onPickAmbiguous?: (skillId: string) => void;
  /** Plan 05-03 — invoked when the user dismisses the ambiguity picker. */
  onDismissAmbiguous?: () => void;
}

function formatScore(score: number): string {
  return score.toFixed(2);
}

export default function RoutingTracePart({ part, onPickAmbiguous, onDismissAmbiguous }: RoutingTracePartProps) {
  const payload = (part as RoutingPartType).data;
  const skillScores: SkillScore[] = payload?.skillScores ?? [];
  const ambiguous: boolean = payload?.ambiguous === true;
  const decision = payload?.decision;

  // Only render for route decisions (or counter-evidence which has the
  // same substrate). For other decisions the dispatcher routes elsewhere
  // or drops, so this guard is defensive.
  const isRouted = decision === 'route' || decision === 'counter_evidence';

  if (!isRouted) {
    return null;
  }

  return (
    <div
      data-testid="routing-trace"
      data-decision={decision}
      data-ambiguous={String(ambiguous)}
      className="my-2 rounded-md border-l-4 border-l-primary bg-surface-2 px-3 py-2 font-mono text-xs"
    >
      <div className="flex items-center gap-2 text-text-muted">
        <span className="font-semibold text-primary">路由分析</span>
      </div>
      {skillScores.length === 0 ? (
        <div
          data-testid="routing-trace-empty"
          className="mt-1 text-text-muted"
        >
          路由信息缺失。
        </div>
      ) : (
        <ul className="mt-1 space-y-0.5">
          {skillScores.map((s) => (
            <li
              key={s.skillId}
              data-testid="routing-trace-row"
              data-skill-id={s.skillId}
              data-selected={String(s.selected)}
              className={s.selected ? 'text-primary font-semibold' : 'text-text-muted'}
            >
              <span>{s.skillName}</span>
              <span className="ml-2">cos {formatScore(s.score)}</span>
            </li>
          ))}
        </ul>
      )}
      {ambiguous && skillScores.length > 0 && (
        <div
          data-testid="routing-trace-ambiguity"
          className="mt-2 border-t border-border pt-2"
        >
          <SkillPicker
            scores={skillScores}
            onPickAmbiguous={onPickAmbiguous}
            onDismissAmbiguous={onDismissAmbiguous}
          />
        </div>
      )}
    </div>
  );
}
