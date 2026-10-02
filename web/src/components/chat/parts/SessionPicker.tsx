// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SessionPicker — session dropdown (Open Design redesign B3: lives in
// the TopBar; formerly the chat-panel header sibling). The list is the
// LIVE session surface (useAppStore.sessions, UI-local per D5-IA-03 —
// messages stay owned by the per-session Chat instances). Selecting a
// row dispatches setActiveSession; the foot offers 新建会话; each row
// exposes a hover × delete (IN-01 interaction moved from the deleted
// LeftSidebar).
import { useEffect, useRef, useState } from 'react';
import { ChevronDown, MessageCircle, Plus } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { useAppStore } from '../../../store/app-store';

export default function SessionPicker() {
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = sessions.find((s) => s.id === activeSessionId);

  useEffect(() => {
    if (!open) return;
    function onMouseDown(e: MouseEvent) {
      const target = e.target as Node | null;
      if (!target || !ref.current) return;
      if (!ref.current.contains(target)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [open]);

  return (
    <div data-testid="session-picker" className="relative" ref={ref}>
      <button
        type="button"
        data-testid="session-picker-trigger"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex w-full items-center justify-between gap-2 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text"
      >
        <span className="flex min-w-0 items-center gap-1.5">
          <MessageCircle className="h-3.5 w-3.5 shrink-0 text-primary" />
          <span className="truncate">{current?.name ?? '选择会话'}</span>
        </span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0" />
      </button>

      {open && (
        <div
          data-testid="session-picker-menu"
          className="absolute z-10 mt-1 w-full min-w-52 rounded-md border border-border bg-surface-2 shadow-lg"
        >
          {sessions.map((session) => {
            const active = session.id === activeSessionId;
            return (
              <div key={session.id} className="group relative">
                <button
                  type="button"
                  data-testid={active ? 'session-picker-item-active' : 'session-picker-item'}
                  data-session-id={session.id}
                  onClick={() => {
                    useAppStore.getState().setActiveSession(session.id);
                    setOpen(false);
                  }}
                  className={cn(
                    'block w-full px-2 py-1 pr-7 text-left text-xs hover:bg-surface-3',
                    active && 'bg-primary-dim',
                  )}
                >
                  <span className="block truncate">{session.name}</span>
                </button>
                <button
                  type="button"
                  data-testid="session-picker-item-delete"
                  data-session-id={session.id}
                  onClick={(e) => {
                    e.stopPropagation();
                    useAppStore.getState().deleteSession(session.id);
                  }}
                  aria-label={`删除会话 ${session.name}`}
                  className="absolute right-1 top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-md text-text-muted opacity-0 hover:bg-surface-3 hover:text-text group-hover:opacity-100"
                >
                  ×
                </button>
              </div>
            );
          })}
          <button
            type="button"
            data-testid="session-picker-new-session"
            onClick={() => {
              useAppStore.getState().createSession();
              setOpen(false);
            }}
            className="block w-full border-t border-border px-2 py-1 text-left text-xs text-primary hover:bg-surface-3"
          >
            <span className="inline-flex items-center gap-1">
              <Plus className="h-3 w-3" />
              新建会话
            </span>
          </button>
          {/* Session-management phase — the drawer holds the full roster
              surface (search/groups/pin/rename); this quick switcher
              stays a shortcut. */}
          <button
            type="button"
            data-testid="session-picker-open-drawer"
            onClick={() => {
              useAppStore.getState().setSessionPanelOpen(true);
              setOpen(false);
            }}
            className="block w-full border-t border-border px-2 py-1 text-left text-xs text-text-muted hover:bg-surface-3"
          >
            查看全部会话
          </button>
        </div>
      )}
    </div>
  );
}
