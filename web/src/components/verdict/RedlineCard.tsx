// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 — SC-7 / CMPL-07 / D5-14.
//
// RedlineCard — the clause-level rewrite proposal UI. Renders AFTER
// CounterEvidenceCard (when both present) in the chat bubble. The card
// is ABSENT on 合规 verdicts (the BE does not emit a data-redline chunk
// for 合规 — D5-14 cost control; the FE does NOT render an empty state
// because absence is correct).
//
// Layout:
//   - 原条款:     originalClause text in --color-redline-removed bg block,
//                 strikethrough in --color-status-non-compliant, mono font.
//   - 建议改写:   proposedRewrite text in --color-redline-added bg block,
//                 in --color-status-compliant, mono font.
//   - 修改理由:   rationale text in a block with 2px teal left-border
//                 (--color-annotation), 修改理由 label in teal.
//   - CTAs:       复制改写文本 (copy proposedRewrite → toast 已复制到剪贴板。)
//                 下载为 .md (download the diff as a markdown file).
//
// Threats mitigated:
//   - T-5-28: no dangerouslySetInnerHTML — diff text rendered via React
//     text-node default (never inject HTML).
//   - T-5-29 (accept): the proposed_rewrite might echo the original — the
//     operator reviews before any action; v1 does NOT write back to the
//     authority doc. Copy + download are explicit user actions.
//
// Wire shape: receives a `data: RedlinePayload` prop (from the
// data-redline chunk parsed by MessageBubble). The V8 required-field
// enforcement lives at wire.RedlinePayload.UnmarshalJSON on the BE side;
// the TS type mirrors the required shape so a truncated payload would
// fail the (p: RedlinePayload) => void consumer signature at the
// dispatcher.
import { useState } from 'react';
import { Check, Copy, Download, FileDiff } from 'lucide-react';
import { cn } from '../../lib/cn';
import { copyToClipboard, downloadMarkdown } from '../../lib/clipboard';
import type { RedlinePayload } from '../../types/wire-types';

export interface RedlineCardProps {
  data: RedlinePayload;
}

const TOAST_TEXT = '已复制到剪贴板。';

export default function RedlineCard({ data }: RedlineCardProps) {
  const [toast, setToast] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    const ok = await copyToClipboard(data.proposedRewrite);
    if (ok) {
      setCopied(true);
      setToast(TOAST_TEXT);
      window.setTimeout(() => {
        setToast(null);
        setCopied(false);
      }, 2000);
    } else {
      setToast('复制失败。请手动选择文本复制。');
      window.setTimeout(() => setToast(null), 3000);
    }
  }

  function handleDownload() {
    const filename = (data.clauseId ? data.clauseId : 'redline') + '.md';
    const body = `# 条款修订建议

## 原条款

${data.originalClause}

## 建议改写

${data.proposedRewrite}

## 修改理由

${data.rationale}
`;
    downloadMarkdown(filename, body);
  }

  return (
    <div
      data-testid="redline-card"
      className="my-2 rounded-lg border-0 border-l-4 border-l-border bg-surface pl-4 pr-5 py-3 text-text"
    >
      <div className="flex items-center gap-2">
        <FileDiff size={16} className="text-text-muted" aria-hidden="true" />
        <span className="text-base font-semibold text-text">条款修订建议</span>
      </div>

      {/* 原条款 — strikethrough red-tinted bg block, line-through status-non-compliant, mono */}
      <div className="mt-3">
        <div className="text-xs font-semibold text-text-muted">原条款</div>
        <pre
          data-testid="redline-original"
          className={cn(
            'mt-1 overflow-x-auto whitespace-pre-wrap rounded-md p-2 font-mono text-xs',
            'bg-redline-removed line-through text-status-non-compliant',
          )}
        >
          {data.originalClause}
        </pre>
      </div>

      {/* 建议改写 — green-tinted bg block, status-compliant, mono */}
      <div className="mt-3">
        <div className="text-xs font-semibold text-text-muted">建议改写</div>
        <pre
          data-testid="redline-rewrite"
          className={cn(
            'mt-1 overflow-x-auto whitespace-pre-wrap rounded-md p-2 font-mono text-xs',
            'bg-redline-added text-status-compliant',
          )}
        >
          {data.proposedRewrite}
        </pre>
      </div>

      {/* 修改理由 — teal label + teal left-border block */}
      <div className="mt-3">
        <div className="text-xs font-semibold text-annotation">修改理由</div>
        <div
          data-testid="redline-rationale"
          className="mt-1 rounded-r-md border-l-2 border-l-annotation bg-surface-2 px-3 py-2 text-base text-text"
        >
          {data.rationale}
        </div>
      </div>

      {/* CTAs */}
      <div className="mt-3 flex items-center gap-2">
        <button
          type="button"
          data-testid="redline-copy-cta"
          onClick={() => {
            void handleCopy();
          }}
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs hover:bg-surface-3"
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          <span>{copied ? '已复制' : '复制改写文本'}</span>
        </button>
        <button
          type="button"
          data-testid="redline-download-cta"
          onClick={handleDownload}
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs hover:bg-surface-3"
        >
          <Download size={12} />
          <span>下载为 .md</span>
        </button>
      </div>

      {/* Inline toast — matches the wizard-success-toast pattern (no sonner dep). */}
      {toast && (
        <div
          data-testid="redline-toast"
          role="status"
          aria-live="polite"
          className="mt-2 text-xs text-text-muted"
        >
          {toast}
        </div>
      )}
    </div>
  );
}
