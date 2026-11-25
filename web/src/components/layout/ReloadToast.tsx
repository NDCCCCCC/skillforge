// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ReloadToast — Plan 05-03 / SC-2 (SYSK-04) toast surface.
//
// Subscribes to `store.skillReloadStatus` and renders a sonner-style
// notification whenever a new entry arrives. Per Copywriting Contract:
//
//   success (status='ok'):    `Skill 已重载：{skillName}`  auto-dismiss 4s
//   failure (status='failed'): `Skill 加载失败，已保留旧版本：{skillName}`
//                              + body `原因：{error}`  persists
//
// We don't depend on the `sonner` package (which would add a dependency);
// the toast is rendered via a lightweight portal-free inline surface
// inside the App shell. Phase 5/05-05 can swap in `sonner` when shadcn
// init runs and the registry includes the toast primitive.
//
// Threat model:
//   - T-5-14 — validation error strings are rendered as plain text.
//     If the BE error string includes an absolute filesystem path,
//     the toast strips it to the basename (defense-in-depth).
//
// Mount at the App root.
import { useEffect } from 'react';
import { CheckCircle2, FileWarning, X } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import type { SkillReloadEntry } from '../../store/app-store';
import { stripFsPath } from '../../lib/stripFsPath';

const SUCCESS_AUTO_DISMISS_MS = 4000;

interface ToastItemProps {
  entry: SkillReloadEntry;
  skillId: string;
  onDismiss: (skillId: string) => void;
}

function ToastItem({ entry, skillId, onDismiss }: ToastItemProps) {
  const failed = entry.status === 'failed';

  // Quick 261125-mk7 — per-item auto-dismiss. The timer lives HERE, keyed
  // on the entry, not in a parent effect over the whole status map: the old
  // parent implementation stored all setTimeout handles in one array and
  // cleared them on every `skillReloadStatus` change, then re-skipped the
  // already-seen entries via the ts watermark — so a second reload arriving
  // within the 4s window (two toasts stacking, the live UAT symptom) killed
  // the first toast's timer without ever re-creating it, and success toasts
  // hung forever. Ownership per item survives sibling arrivals.
  useEffect(() => {
    if (entry.status !== 'ok') return;
    const t = setTimeout(() => onDismiss(skillId), SUCCESS_AUTO_DISMISS_MS);
    return () => clearTimeout(t);
  }, [entry, skillId, onDismiss]);

  const borderClass = failed
    ? 'border-status-non-compliant'
    : 'border-status-compliant';
  const Icon = failed ? FileWarning : CheckCircle2;
  const iconColor = failed ? 'text-status-non-compliant' : 'text-status-compliant';

  return (
    <div
      data-testid={failed ? 'reload-toast-failed' : 'reload-toast-success'}
      data-skill-id={skillId}
      className={`pointer-events-auto rounded-md border-l-4 ${borderClass} bg-surface-2 px-3 py-2 shadow-lg`}
    >
      <div className="flex items-start gap-2">
        <Icon className={`h-4 w-4 mt-0.5 shrink-0 ${iconColor}`} />
        <div className="flex-1 min-w-0">
          <div className="text-xs font-semibold text-text">
            {failed
              ? `Skill 加载失败，已保留旧版本：${entry.skillName}`
              : `Skill 已重载：${entry.skillName}`}
          </div>
          {failed && entry.error && (
            <div className="mt-1 text-xs text-text-muted">
              原因：{stripFsPath(entry.error)}
            </div>
          )}
        </div>
        <button
          type="button"
          data-testid="reload-toast-dismiss"
          onClick={() => onDismiss(skillId)}
          className="rounded-md p-1 text-text-muted hover:bg-surface-3 hover:text-text"
          aria-label="Dismiss toast"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

export default function ReloadToast() {
  const skillReloadStatus = useAppStore((s) => s.skillReloadStatus);
  const clearSkillReloadStatus = useAppStore((s) => s.clearSkillReloadStatus);

  const entries = Object.entries(skillReloadStatus);
  if (entries.length === 0) return null;

  return (
    <div
      data-testid="reload-toast-stack"
      className="pointer-events-none fixed top-3 right-3 z-50 flex flex-col gap-2 w-80"
    >
      {entries.map(([skillId, entry]) => (
        <ToastItem key={skillId} entry={entry} skillId={skillId} onDismiss={clearSkillReloadStatus} />
      ))}
    </div>
  );
}
