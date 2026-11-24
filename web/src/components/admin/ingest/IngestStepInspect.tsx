// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- the Variable Inspect panel that the timeline's
// expanded row hosts (55-UI-SPEC Component 5 expanded panel / Dify
// Variable Inspect mode).
//
// SELF-CONTAINED PROPS (wave-1 isolation): this component does NOT import
// ./types. Every value arrives already derived/humanized from the caller
// (55-03 wires it from job.steps). Keeping derivation in exactly one
// place (types.ts) is the point -- this file only renders.
//
// Wire degradation: `summary` and `skipReason` are forward-compat props.
// Absent summary -> the 输入摘要 block is NOT rendered (the caller
// normally passes undefined because the 54 wire does not emit it). The
// FE never fabricates these values.
import { cn } from '../../../lib/cn';

/** 8 known summary keys -> zh labels (55-UI-SPEC §Summary Key Labels,
 * verbatim). An unknown key renders its RAW key + value (never blank). */
const SUMMARY_KEY_LABELS: Record<string, string> = {
  parsed_chars: '产物字数',
  chunk_count: '块数',
  embedded_count: '向量数',
  embedded_failed: '失败块数',
  inserted_rows: '入库行数',
  entity_count: '抽取实体数',
  triples_written: '三元组写入数',
  purged_before_insert: '插入前清理行数',
};

export interface IngestStepInspectProps {
  /** zh label supplied by the caller (STEP_LABEL[step] ?? step). */
  stepLabel: string;
  /** forward-compat: absent -> no 输入摘要 block. */
  summary?: Record<string, number | string>;
  /** absent -> 产物定位 renders '—'. */
  artifactRef?: string;
  attempt?: number;
  /** failed step -> rendered verbatim in the danger color. */
  error?: string;
  /** skipped step -> its reason, rendered muted and verbatim. */
  skipReason?: string;
  /** true -> the 确定性错误 annotation row is added. */
  deadNote?: boolean;
  /** true -> the ECL cross-link row is rendered. */
  isEclStep?: boolean;
  onGoEcl: () => void;
  /** true -> the idempotency hint row is rendered. */
  showRetryHint?: boolean;
}

export default function IngestStepInspect({
  stepLabel,
  summary,
  artifactRef,
  attempt,
  error,
  skipReason,
  deadNote,
  isEclStep,
  onGoEcl,
  showRetryHint,
}: IngestStepInspectProps) {
  const summaryEntries =
    summary && Object.keys(summary).length > 0 ? Object.entries(summary) : null;

  return (
    <div data-testid="ingest-step-inspect" className="space-y-2">
      <div className="text-xs font-semibold text-text">{stepLabel}</div>

      {summaryEntries && (
        <div data-testid="ingest-step-summary" className="space-y-1">
          <div className="text-xs font-semibold text-text">输入摘要</div>
          {summaryEntries.map(([key, value]) => (
            <div key={key} className="flex items-baseline gap-2 text-xs">
              <span className="text-text-muted">{SUMMARY_KEY_LABELS[key] ?? key}</span>
              <span className="font-mono text-text">{String(value)}</span>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-baseline gap-2 text-xs">
        <span className="text-text-muted">产物定位</span>
        <span
          data-testid="ingest-step-artifact"
          title={artifactRef}
          className="max-w-full truncate font-mono text-text"
        >
          {artifactRef ?? '—'}
        </span>
      </div>

      <div className="flex items-baseline gap-2 text-xs">
        <span className="text-text-muted">尝试</span>
        <span className="font-mono text-text">{attempt ?? 0}</span>
      </div>

      {deadNote && (
        <div
          data-testid="ingest-step-dead-note"
          className="text-xs text-status-needs-info"
        >
          确定性错误 — 不自动重试
        </div>
      )}

      {showRetryHint && (
        <div data-testid="ingest-step-retry-hint" className="text-xs text-text-muted">
          重试从上游最近成功步骤的产物继续，下游步骤将标记待重跑
        </div>
      )}

      {error && (
        <p
          data-testid="ingest-step-error"
          className={cn('whitespace-pre-wrap text-xs text-status-non-compliant')}
        >
          {error}
        </p>
      )}

      {skipReason && (
        <p data-testid="ingest-step-skip-reason" className="text-xs text-text-muted">
          {skipReason}
        </p>
      )}

      {isEclStep && (
        <button
          type="button"
          data-testid="ingest-step-ecl-link"
          onClick={onGoEcl}
          className="text-xs text-primary hover:underline"
        >
          ECL 全局状态见 ECL 管线 tab
        </button>
      )}
    </div>
  );
}
