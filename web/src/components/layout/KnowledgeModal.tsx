// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeModal — Open Design redesign B3. The full-screen-ish corpus
// browser (880px centered card) that replaces the deleted LeftSidebar
// 文件 block:
//   - Left rail (268px): the 4-tier disk-truth tree from GET
//     /api/knowledge/docs (logic migrated verbatim from LeftSidebar —
//     tier headers always render with the directory hint; docs load
//     per tier; fetch failure is non-fatal).
//   - Right pane: DocViewer preview of the selected doc (the same
//     center/DocViewer component, kind='doc-view' payload).
//
// Threat model: doc filenames come from the backend and are rendered
// as text only (no dangerouslySetInnerHTML, no client-side path
// construction) — the LeftSidebar posture carries over unchanged.
//
// B7 fix: the modal renders through createPortal(document.body). It is
// mounted inside TopBar, and `.topbar` carries backdrop-filter — per
// the CSS containing-block rules that makes the bar the containing
// block for its FIXED descendants, so the scrim inset-0 covered only
// the 56px bar and the card anchored to it instead of the viewport.
// Portaling out of that subtree restores fixed = viewport.
import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, X } from 'lucide-react';
import { cn } from '../../lib/cn';
import DocViewer from '../center/DocViewer';
import type { Tier } from '../../types/verdict';

interface KnowledgeDoc {
  filename: string;
  bytes: number;
}
interface KnowledgeTier {
  tier: string;
  docs: KnowledgeDoc[];
}

const TIER_ORDER: Tier[] = ['权威', '历史', '个人', '草稿'];

// Directory hint per tier — mirrors the on-disk layout (knowledge/<en>/).
const TIER_DIR: Record<string, string> = {
  权威: 'knowledge/authority/',
  历史: 'knowledge/historical/',
  个人: 'knowledge/personal/',
  草稿: 'knowledge/draft/',
};

export interface KnowledgeModalProps {
  open: boolean;
  onClose: () => void;
}

export default function KnowledgeModal({ open, onClose }: KnowledgeModalProps) {
  const [docsByTier, setDocsByTier] = useState<Record<string, KnowledgeDoc[]>>({});
  const [openTiers, setOpenTiers] = useState<Record<string, boolean>>({ 权威: true });
  const [selected, setSelected] = useState<{ tier: Tier; file: string } | null>(null);

  // Fetch the disk-truth corpus tree when the modal opens. Failure is
  // non-fatal — tier headers + dir hints still render.
  useEffect(() => {
    if (!open) return;
    const ctl = new AbortController();
    fetch('/api/knowledge/docs', { signal: ctl.signal })
      .then(async (r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<{ tiers: KnowledgeTier[] }>;
      })
      .then((body) => {
        const next: Record<string, KnowledgeDoc[]> = {};
        for (const t of body.tiers ?? []) next[t.tier] = t.docs ?? [];
        setDocsByTier(next);
      })
      .catch(() => {
        // Offline backend — keep the tier headers + dir hints.
      });
    return () => ctl.abort();
  }, [open]);

  // Esc closes the modal.
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;

  function toggleTier(tier: string) {
    setOpenTiers((prev) => ({ ...prev, [tier]: !prev[tier] }));
  }

  return createPortal(
    <div
      data-testid="knowledge-modal-scrim"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-6"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        data-testid="knowledge-modal"
        role="dialog"
        aria-modal="true"
        aria-label="知识库浏览"
        className="flex h-[min(640px,80vh)] w-[min(880px,92vw)] flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        <div className="flex h-11 flex-none items-center justify-between border-b border-border px-4">
          <span className="text-sm font-semibold tracking-wide text-text">知识库</span>
          <button
            type="button"
            data-testid="knowledge-modal-close"
            onClick={onClose}
            className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
            aria-label="关闭知识库"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="flex min-h-0 flex-1">
          <div
            data-testid="knowledge-modal-tree"
            className="w-[268px] flex-none overflow-y-auto border-r border-border p-2"
          >
            {TIER_ORDER.map((tier) => {
              const docs = docsByTier[tier] ?? [];
              const isOpen = Boolean(openTiers[tier]);
              return (
                <div key={tier} className="mb-1">
                  <button
                    type="button"
                    data-testid="knowledge-modal-tier"
                    data-tier={tier}
                    onClick={() => toggleTier(tier)}
                    className="flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-xs font-semibold text-text hover:bg-surface-2"
                  >
                    {isOpen ? (
                      <FolderOpen className="h-3.5 w-3.5 shrink-0 text-primary" />
                    ) : (
                      <Folder className="h-3.5 w-3.5 shrink-0 text-primary" />
                    )}
                    <span>{tier}</span>
                    <span className="ml-auto font-mono text-[10px] text-text-dim">{docs.length}</span>
                    {isOpen ? (
                      <ChevronDown className="h-3 w-3 text-text-muted" />
                    ) : (
                      <ChevronRight className="h-3 w-3 text-text-muted" />
                    )}
                  </button>
                  {isOpen && (
                    <ul className="ml-3 border-l border-border pl-1">
                      {docs.length === 0 ? (
                        <li className="px-2 py-1 font-mono text-[10px] text-text-dim">
                          {TIER_DIR[tier] ?? tier}
                        </li>
                      ) : (
                        docs.map((doc) => {
                          const active = selected?.tier === tier && selected.file === doc.filename;
                          return (
                            <li key={doc.filename}>
                              <button
                                type="button"
                                data-testid="knowledge-modal-doc"
                                data-tier={tier}
                                data-file={doc.filename}
                                onClick={() => setSelected({ tier, file: doc.filename })}
                                className={cn(
                                  'flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-left text-xs text-text-muted hover:bg-surface-2 hover:text-text',
                                  active && 'bg-primary-dim text-text',
                                )}
                              >
                                <FileText className="h-3 w-3 shrink-0" />
                                <span className="truncate">{doc.filename}</span>
                              </button>
                            </li>
                          );
                        })
                      )}
                    </ul>
                  )}
                </div>
              );
            })}
          </div>
          <div className="min-w-0 flex-1 overflow-y-auto">
            {selected ? (
              <DocViewer tier={selected.tier} file={selected.file} />
            ) : (
              <div className="flex h-full flex-col items-center justify-center text-center text-text-muted">
                <Folder className="mb-3 h-8 w-8 text-text-dim" />
                <div className="text-sm">选择左侧文档以预览</div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
