// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SessionDrawer — session-management phase. The left session panel's
// CONTENT (header / search / grouped roster). Positioning is owned by
// ChatLayout's `.session-panel-wrap` — the same inline-panel pattern
// as the right DetailPanel: ≥1280px the panel is an inline column
// (chat compresses but stays fully usable); <1280px the CSS flips it
// to a fixed overlay with a scrim. This component renders the inner
// flex column only, so it works in both forms.
//
// Surfaces:
//   - Roster from the Zustand store (hydrated from GET /api/sessions on
//     App mount; locally-created sessions append normally)
//   - Front-end search (name contains, case-insensitive)
//   - Time groups: 置顶 / 今天 / 7 天内 / 更早 keyed on
//     `lastActiveAt ?? createdAt`
//   - Row actions: pin toggle (persisted via store), inline rename
//     (optimistic store write + PATCH; failure reverts + sf-toast),
//     delete (store roster removal + DELETE; the store falls the
//     active session back when the deleted one was active)
//
// Row actions are ALWAYS visible on touch (opacity-100) and
// hover-revealed on md+ (md:opacity-0 md:group-hover:opacity-100).
import { useEffect, useMemo, useState } from 'react';
import { Pencil, Pin, PinOff, Plus, Search, Trash2, X } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore, type SessionMeta } from '../../store/app-store';
import { deleteSession as deleteSessionAPI, renameSession as renameSessionAPI } from '../../lib/sessions';
import ApprovalRulesPanel from '../ApprovalRulesPanel';
import PlanRulesPanel from '../PlanRulesPanel';

export interface SessionDrawerProps {
  open: boolean;
  onClose: () => void;
}

/** Group bucket for one session row. */
type GroupKey = 'pinned' | 'today' | 'week' | 'older';

const GROUP_LABEL: Record<GroupKey, string> = {
  pinned: '置顶',
  today: '今天',
  week: '7 天内',
  older: '更早',
};
const GROUP_ORDER: GroupKey[] = ['pinned', 'today', 'week', 'older'];

/** Relative-time label for a session row (Chinese-first copy). */
function relativeTime(ts: number, now: number): string {
  const diff = now - ts;
  if (diff < 60_000) return '刚刚';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  const d = new Date(ts);
  const today = new Date(now);
  if (d.toDateString() === today.toDateString()) {
    return `今天 ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`;
  }
  if (diff < 7 * 86_400_000) return `${Math.floor(diff / 86_400_000)} 天前`;
  const y = d.getFullYear();
  const m = (d.getMonth() + 1).toString().padStart(2, '0');
  const day = d.getDate().toString().padStart(2, '0');
  return y === today.getFullYear() ? `${m}-${day}` : `${y}-${m}-${day}`;
}

/** Bucket one session by pin + recency (now injectable for tests). */
function groupOf(s: SessionMeta, now: number): GroupKey {
  if (s.pinned) return 'pinned';
  const ts = s.lastActiveAt ?? s.createdAt;
  const diff = now - ts;
  if (diff < 0) return 'today'; // future clock skew → freshest bucket
  if (diff < 86_400_000) return 'today';
  if (diff < 7 * 86_400_000) return 'week';
  return 'older';
}

export default function SessionDrawer({ open, onClose }: SessionDrawerProps) {
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const setActiveSession = useAppStore((s) => s.setActiveSession);
  const createSession = useAppStore((s) => s.createSession);
  const deleteSessionInStore = useAppStore((s) => s.deleteSession);
  const renameSessionInStore = useAppStore((s) => s.renameSession);
  const toggleSessionPin = useAppStore((s) => s.toggleSessionPin);

  const [query, setQuery] = useState('');
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameDraft, setRenameDraft] = useState('');

  // Esc closes the drawer.
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  // Reset the transient state whenever the drawer reopens.
  useEffect(() => {
    if (open) {
      setQuery('');
      setRenamingId(null);
    }
  }, [open]);

  // Filter + group + sort (newest-first within each group).
  const groups = useMemo(() => {
    const now = Date.now();
    const q = query.trim().toLowerCase();
    const filtered = q
      ? sessions.filter((s) => s.name.toLowerCase().includes(q))
      : sessions;
    const byGroup = new Map<GroupKey, SessionMeta[]>();
    for (const key of GROUP_ORDER) byGroup.set(key, []);
    for (const s of filtered) {
      byGroup.get(groupOf(s, now))!.push(s);
    }
    for (const key of GROUP_ORDER) {
      byGroup.get(key)!.sort(
        (a, b) => (b.lastActiveAt ?? b.createdAt) - (a.lastActiveAt ?? a.createdAt),
      );
    }
    return GROUP_ORDER.map((key) => ({
      key,
      label: GROUP_LABEL[key],
      rows: byGroup.get(key)!,
    })).filter((g) => g.rows.length > 0);
  }, [sessions, query]);

  // B9 — no render gate: the drawer stays mounted (inside the
  // always-mounted .session-panel-wrap) so the wrap's exit slide
  // paints real content and the roster keeps its scroll position.
  // Visibility/positioning/focus-blocking are owned by the wrap's
  // data-open + inert; `open` still drives the Esc listener above and
  // the transient-state reset on reopen.

  function handleSelect(id: string) {
    if (renamingId === id) return; // editing — don't steal focus/clobber
    setActiveSession(id);
    onClose();
  }

  function handleCreate() {
    createSession();
    onClose();
  }

  function startRename(s: SessionMeta) {
    setRenamingId(s.id);
    setRenameDraft(s.name);
  }

  function commitRename(id: string) {
    const title = renameDraft.trim();
    const prev = useAppStore.getState().sessions.find((s) => s.id === id);
    setRenamingId(null);
    if (!title || title === prev?.name) return;
    // Optimistic FE write; PATCH failure reverts + toasts.
    renameSessionInStore(id, title);
    void renameSessionAPI(id, title).then((ok) => {
      if (!ok) {
        renameSessionInStore(id, prev?.name ?? '');
        window.dispatchEvent(
          new CustomEvent('sf-toast', {
            detail: { kind: 'error', message: '重命名失败，已恢复原名称' },
          }),
        );
      }
    });
  }

  function handleDelete(id: string) {
    deleteSessionInStore(id); // store falls active back if needed
    if (renamingId === id) setRenamingId(null);
    void deleteSessionAPI(id);
  }

  return (
    <div
      data-testid="session-drawer"
      className="session-drawer flex h-full min-h-0 flex-col"
    >
        {/* Header — title + 新建 + close */}
        <div className="flex h-11 flex-none items-center justify-between border-b border-border px-4">
          <span className="text-sm font-semibold tracking-wide text-text">会话</span>
          <div className="flex items-center gap-1">
            <button
              type="button"
              data-testid="session-drawer-create"
              onClick={handleCreate}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
              aria-label="新建会话"
              title="新建会话"
            >
              <Plus className="h-4 w-4" />
            </button>
            <button
              type="button"
              data-testid="session-drawer-close"
              onClick={onClose}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
              aria-label="关闭会话列表"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="flex-none border-b border-border px-3 py-2">
          <div className="flex items-center gap-2 rounded-md border border-border bg-surface-2 px-2 py-1.5">
            <Search className="h-3.5 w-3.5 flex-none text-text-muted" />
            <input
              data-testid="session-drawer-search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索会话…"
              className="w-full bg-transparent text-xs text-text outline-none placeholder:text-text-muted"
            />
          </div>
        </div>

        {/* Grouped roster */}
        <div data-testid="session-drawer-body" className="flex-1 overflow-y-auto px-2 py-2">
          {groups.length === 0 && (
            <p className="px-2 py-6 text-center text-xs text-text-muted">
              {query.trim() ? '没有匹配的会话' : '还没有会话，点击右上角 ＋ 新建'}
            </p>
          )}
          {groups.map((g) => (
            <div key={g.key} data-testid="session-drawer-group" data-group={g.key} className="mb-3">
              <div className="px-2 py-1 text-[11px] font-medium uppercase tracking-wider text-text-muted">
                {g.label}
              </div>
              {g.rows.map((s) => {
                const active = s.id === activeSessionId;
                const renaming = renamingId === s.id;
                return (
                  <div
                    key={s.id}
                    data-testid="session-drawer-row"
                    data-session-id={s.id}
                    className={cn(
                      'group relative flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 transition-colors',
                      active ? 'bg-primary-dim' : 'hover:bg-surface-2',
                    )}
                    onClick={() => handleSelect(s.id)}
                  >
                    {renaming ? (
                      <input
                        data-testid="session-drawer-rename-input"
                        autoFocus
                        value={renameDraft}
                        onChange={(e) => setRenameDraft(e.target.value)}
                        onBlur={() => commitRename(s.id)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') commitRename(s.id);
                          if (e.key === 'Escape') setRenamingId(null);
                        }}
                        onClick={(e) => e.stopPropagation()}
                        className="w-full rounded border border-primary bg-surface px-1.5 py-0.5 text-xs text-text outline-none"
                      />
                    ) : (
                      <>
                        <span
                          className={cn(
                            'min-w-0 flex-1 truncate text-xs',
                            active ? 'font-medium text-text' : 'text-text',
                          )}
                          title={s.name}
                        >
                          {s.name}
                        </span>
                        <span className="flex-none text-[10px] text-text-muted">
                          {relativeTime(s.lastActiveAt ?? s.createdAt, Date.now())}
                        </span>
                      </>
                    )}

                    {/* Row actions — always visible on touch, hover on md+ */}
                    {!renaming && (
                      <div
                        data-testid="session-drawer-actions"
                        className="absolute right-1 top-1/2 flex -translate-y-1/2 items-center gap-0.5 rounded-md bg-surface/90 p-0.5 opacity-100 shadow-sm md:opacity-0 md:group-hover:opacity-100"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <button
                          type="button"
                          data-testid="session-drawer-pin"
                          onClick={() => toggleSessionPin(s.id)}
                          className="inline-flex h-6 w-6 items-center justify-center rounded text-text-muted hover:bg-surface-3 hover:text-text"
                          aria-label={s.pinned ? '取消置顶' : '置顶'}
                          title={s.pinned ? '取消置顶' : '置顶'}
                        >
                          {s.pinned ? <PinOff className="h-3.5 w-3.5" /> : <Pin className="h-3.5 w-3.5" />}
                        </button>
                        <button
                          type="button"
                          data-testid="session-drawer-rename"
                          onClick={() => startRename(s)}
                          className="inline-flex h-6 w-6 items-center justify-center rounded text-text-muted hover:bg-surface-3 hover:text-text"
                          aria-label="重命名"
                          title="重命名"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          data-testid="session-drawer-delete"
                          onClick={() => handleDelete(s.id)}
                          className="inline-flex h-6 w-6 items-center justify-center rounded text-text-muted hover:bg-surface-3 hover:text-status-error"
                          aria-label="删除会话"
                          title="删除会话"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ))}
        </div>

        {/* Plan 11-07 — D11-D4 HITL-08 审批规则 section. The conceptual
            "LeftSidebar 5th section" lands at the foot of this session
            panel (the old LeftSidebar was removed in the Open Design
            redesign; this drawer is the surviving left-side surface).
            The panel owns its own fetch / empty / error / revoke
            states — mounted unconditionally. */}
        <ApprovalRulesPanel />

        {/* Plan 12-07 — D11-D2 single-table reuse. The 6th section
            renders plan_approve-specific approval rules; the BE endpoint
            is shared with Phase 11 ApprovalRulesPanel above (D11-D2
            approval_rules is a single table spanning all HITL targets).
            The FE filter (target='plan_approve') splits responsibility:
            // ApprovalRulesPanel owns memory:facts / review_report; this
            // panel owns plan_approve. */}
        <PlanRulesPanel />
    </div>
  );
}
