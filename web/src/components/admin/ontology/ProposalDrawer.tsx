// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — schema-proposal approval Drawer (D-UI-01..04). Right-side
// overlay (same fixed-overlay pattern as SessionDrawer — the project has
// no shadcn Sheet). Three sections: diff preview (added Turtle lines,
// highlighted), reasoning (source text), and the Approve / Reject footer.
// Approve supports renaming the local name inline (D-Induct-04); a 409
// IRI conflict renders inline instead of a generic error.

import { useState } from 'react';
import { Check, X } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { approveProposal, rejectProposal } from './ontologyApi';
import type { Proposal } from './types';

export interface ProposalDrawerProps {
  proposal: Proposal | null;
  onClose: () => void;
  onDecided: () => void;
}

export default function ProposalDrawer({ proposal, onClose, onDecided }: ProposalDrawerProps) {
  const [customName, setCustomName] = useState('');
  const [busy, setBusy] = useState<'approve' | 'reject' | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState(false);

  if (!proposal) return null;

  async function handleApprove() {
    if (!proposal) return;
    setBusy('approve');
    setError(null);
    try {
      await approveProposal(proposal.id, renaming ? customName.trim() : undefined);
      onDecided();
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(null);
    }
  }

  async function handleReject() {
    if (!proposal) return;
    setBusy('reject');
    setError(null);
    try {
      await rejectProposal(proposal.id);
      onDecided();
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div
      data-testid="proposal-drawer-scrim"
      className="fixed inset-0 z-40 bg-black/40"
      onClick={onClose}
      role="presentation"
    >
      <aside
        data-testid="proposal-drawer"
        role="dialog"
        aria-label={`Schema proposal ${proposal.id}`}
        onClick={(e) => e.stopPropagation()}
        className="absolute right-0 top-0 flex h-full w-full max-w-2xl flex-col border-l border-border bg-surface shadow-xl"
      >
        {/* Sticky header */}
        <header className="flex items-center justify-between border-b border-border px-4 py-3">
          <div>
            <div className="font-mono text-xs text-text-muted">{proposal.id}</div>
            <div className="text-sm font-semibold">
              {proposal.kind} · skf:{proposal.local_name}
            </div>
            <div className="text-[11px] text-text-muted">
              {new Date(proposal.created_at).toLocaleString()}
            </div>
          </div>
          <button
            type="button"
            data-testid="proposal-drawer-close"
            onClick={onClose}
            className="rounded-md border border-border bg-surface-2 p-1 hover:bg-surface-3"
            aria-label="关闭"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        <div className="flex-1 space-y-4 overflow-y-auto px-4 py-4">
          {/* Section 1: diff preview */}
          <section data-testid="proposal-diff">
            <h3 className="mb-2 text-xs font-semibold text-text-muted">
              Diff preview — approve 后追加到 TBox
            </h3>
            <div className="grid gap-2 md:grid-cols-2">
              <div className="rounded-md border border-border bg-surface-2 p-2">
                <div className="mb-1 text-[11px] text-text-muted">Current TBox (不变)</div>
                <div className="font-mono text-[11px] text-text-muted">
                  …现有 TBox 内容保持不变…
                </div>
              </div>
              <div className="rounded-md border border-primary/40 bg-primary-dim p-2">
                <div className="mb-1 text-[11px] font-semibold text-primary">
                  After Approval (+{proposal.proposed_triples.length} 行)
                </div>
                <pre className="overflow-x-auto font-mono text-[11px] leading-relaxed">
                  {proposal.proposed_triples.join('\n')}
                </pre>
              </div>
            </div>
          </section>

          {/* Section 2: reasoning */}
          <section data-testid="proposal-reasoning">
            <h3 className="mb-2 text-xs font-semibold text-text-muted">Reasoning — 提议依据</h3>
            <div className="rounded-md border border-border bg-surface-2 p-2 text-xs">
              <p>
                <span className="text-text-muted">中: </span>
                {proposal.description_zh}
              </p>
              <p className="mt-1">
                <span className="text-text-muted">EN: </span>
                {proposal.description_en}
              </p>
              <details className="mt-2">
                <summary className="cursor-pointer text-[11px] text-text-muted">
                  源文本
                </summary>
                <pre className="mt-1 whitespace-pre-wrap font-mono text-[11px]">
                  {proposal.source_text}
                </pre>
              </details>
            </div>
          </section>

          {/* Section 3: rename (optional) */}
          <section>
            <label className="flex items-center gap-2 text-xs">
              <input
                type="checkbox"
                data-testid="proposal-rename-toggle"
                checked={renaming}
                onChange={(e) => setRenaming(e.target.checked)}
              />
              重命名本地名 (IRI 冲突时可用)
            </label>
            {renaming && (
              <input
                data-testid="proposal-rename-input"
                value={customName}
                onChange={(e) => setCustomName(e.target.value)}
                placeholder={proposal.local_name}
                className="mt-1 w-full rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs"
              />
            )}
          </section>

          {error && (
            <div data-testid="proposal-error" className="rounded-md bg-danger/10 p-2 text-xs text-danger">
              {error}
            </div>
          )}
        </div>

        {/* Footer: Approve / Reject */}
        <footer className="flex items-center justify-end gap-2 border-t border-border px-4 py-3">
          <button
            type="button"
            data-testid="proposal-reject"
            onClick={handleReject}
            disabled={busy !== null || proposal.status !== 'pending'}
            className={cn(
              'inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold',
              'hover:bg-surface-3 disabled:opacity-50',
            )}
          >
            <X className="h-3.5 w-3.5" />
            Reject
          </button>
          <button
            type="button"
            data-testid="proposal-approve"
            onClick={handleApprove}
            disabled={busy !== null || proposal.status !== 'pending'}
            className={cn(
              'inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg',
              'disabled:opacity-50',
            )}
          >
            <Check className="h-3.5 w-3.5" />
            {busy === 'approve' ? 'Approving…' : 'Approve'}
          </button>
        </footer>
      </aside>
    </div>
  );
}
