// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeModal — Open Design redesign B3. The full-screen-ish corpus
// browser (880px centered card) that replaces the deleted LeftSidebar
// 文件 block:
//   - Left rail (268px): the disk-truth tree from GET /api/knowledge/docs
//     (logic migrated verbatim from LeftSidebar — tier headers always
//     render with the directory hint; docs load per tier; fetch failure
//     is non-fatal).
//   - Right pane: DocViewer preview of the selected doc (the same
//     center/DocViewer component, kind='doc-view' payload).
//
// Phase 26 (26-08 residue R-1): the tier block set and ORDER are the
// server's (the registry's active slugs — KBM-01 single source), never a
// hardcoded FE list: an admin-created base appears here without an FE
// change, and a deactivated base disappears with it. Labels go through
// useTierLabel (registry display name, bracketed — the KNOW-05 badge
// form); the offline posture renders the seed slugs from kb-bases (the
// one whitelisted FE fallback home). The slug doubles as the directory
// name under knowledge/ (backend contract), so the empty-tier hint is
// computed, not mapped.
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
import { TIER_FALLBACK_LABELS, useTierLabel } from '../../lib/kb-bases';

interface KnowledgeDoc {
  filename: string;
  bytes: number;
}
interface KnowledgeTier {
  tier: string;
  docs: KnowledgeDoc[];
}

// Offline posture (docs fetch failed): the 4 baseline seed slugs, in
// seed order, derived from kb-bases — the FE's single whitelisted
// fallback home. The live list ALWAYS comes from the endpoint.
const SEED_TIERS: KnowledgeTier[] = Object.keys(TIER_FALLBACK_LABELS).map((slug) => ({
  tier: slug,
  docs: [],
}));

export interface KnowledgeModalProps {
  open: boolean;
  onClose: () => void;
}

export default function KnowledgeModal({ open, onClose }: KnowledgeModalProps) {
  const [tiers, setTiers] = useState<KnowledgeTier[]>(SEED_TIERS);
  const [openTiers, setOpenTiers] = useState<Record<string, boolean>>({});
  const [selected, setSelected] = useState<{ tier: string; file: string } | null>(null);

  // Fetch the disk-truth corpus tree when the modal opens. Failure is
  // non-fatal — the seed tier headers + dir hints still render. An OK
  // but EMPTY tier list is a real answer (empty registry, degraded on
  // purpose) and renders an empty tree — it must not resurrect the seed
  // list. The first resolved tier defaults open unless the operator
  // already opened one (their choice wins).
  useEffect(() => {
    if (!open) return;
    const ctl = new AbortController();
    fetch('/api/knowledge/docs', { signal: ctl.signal })
      .then(async (r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<{ tiers: KnowledgeTier[] }>;
      })
      .then((body) => {
        const next = body.tiers ?? [];
        setTiers(next);
        setOpenTiers((prev) =>
          Object.values(prev).some(Boolean) || next.length === 0 ? prev : { [next[0].tier]: true },
        );
      })
      .catch(() => {
        // Offline backend — keep the seed tier headers + dir hints.
        setOpenTiers((prev) =>
          Object.values(prev).some(Boolean) ? prev : { [SEED_TIERS[0].tier]: true },
        );
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
            {tiers.length === 0 && (
              <div data-testid="knowledge-modal-empty" className="px-2 py-1 text-xs text-text-dim">
                暂无知识库
              </div>
            )}
            {tiers.map((t) => (
              <TierSection
                key={t.tier}
                tier={t}
                isOpen={Boolean(openTiers[t.tier])}
                activeFile={selected?.tier === t.tier ? selected.file : null}
                onToggle={() => toggleTier(t.tier)}
                onSelectDoc={(file) => setSelected({ tier: t.tier, file })}
              />
            ))}
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

// TierSection — one tier block of the left rail. A child component (not
// an inline map body) so useTierLabel is a legal per-tier hook call —
// the same shape GraphTree's TierSection uses.
function TierSection({
  tier,
  isOpen,
  activeFile,
  onToggle,
  onSelectDoc,
}: {
  tier: KnowledgeTier;
  isOpen: boolean;
  activeFile: string | null;
  onToggle: () => void;
  onSelectDoc: (file: string) => void;
}) {
  // Registry display label (bracketed badge form); `[slug]` when the
  // projection is unavailable or the slug is unknown.
  const label = useTierLabel(tier.tier);

  return (
    <div className="mb-1">
      <button
        type="button"
        data-testid="knowledge-modal-tier"
        data-tier={tier.tier}
        onClick={onToggle}
        className="flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-xs font-semibold text-text hover:bg-surface-2"
      >
        {isOpen ? (
          <FolderOpen className="h-3.5 w-3.5 shrink-0 text-primary" />
        ) : (
          <Folder className="h-3.5 w-3.5 shrink-0 text-primary" />
        )}
        <span>{label}</span>
        <span className="ml-auto font-mono text-[10px] text-text-dim">{tier.docs.length}</span>
        {isOpen ? (
          <ChevronDown className="h-3 w-3 text-text-muted" />
        ) : (
          <ChevronRight className="h-3 w-3 text-text-muted" />
        )}
      </button>
      {isOpen && (
        <ul className="ml-3 border-l border-border pl-1">
          {tier.docs.length === 0 ? (
            <li className="px-2 py-1 font-mono text-[10px] text-text-dim">
              {`knowledge/${tier.tier}/`}
            </li>
          ) : (
            tier.docs.map((doc) => {
              const active = activeFile === doc.filename;
              return (
                <li key={doc.filename}>
                  <button
                    type="button"
                    data-testid="knowledge-modal-doc"
                    data-tier={tier.tier}
                    data-file={doc.filename}
                    onClick={() => onSelectDoc(doc.filename)}
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
}
