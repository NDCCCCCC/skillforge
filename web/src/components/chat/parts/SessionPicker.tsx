// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SessionPicker — session dropdown that lives in the right chat panel
// header (sibling to SkillPicker). Per the denova-aligned IA, session
// + skill selection both live in the chat; the left sidebar is a
// read-only source library.
//
// Phase 4 ships a hardcoded mock list + local state (no store field).
// Real multi-session management (persistence, server sync) belongs to
// Phase 5; the picker's contract is just "show current + switch".
import { useEffect, useRef, useState } from 'react';
import { ChevronDown, MessageCircle } from 'lucide-react';
import { cn } from '../../../lib/cn';

interface SessionEntry {
  id: string;
  name: string;
}

const SESSIONS: SessionEntry[] = [
  { id: 's1', name: '会话 1' },
  { id: 's2', name: '会话 2' },
];

export default function SessionPicker() {
  const [currentId, setCurrentId] = useState('s1');
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = SESSIONS.find((s) => s.id === currentId);

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
        className="w-full inline-flex items-center justify-between gap-2 rounded-md bg-surface-2 text-text border border-border px-2 py-1 text-xs font-semibold"
      >
        <span className="flex items-center gap-1.5 min-w-0">
          <MessageCircle className="h-3.5 w-3.5 shrink-0 text-primary" />
          <span className="truncate">{current?.name ?? '选择会话'}</span>
        </span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0" />
      </button>

      {open && (
        <div
          data-testid="session-picker-menu"
          className="absolute z-10 mt-1 w-full rounded-md border border-border bg-surface-2 shadow-lg"
        >
          {SESSIONS.map((session) => {
            const active = session.id === currentId;
            return (
              <button
                key={session.id}
                type="button"
                data-testid={active ? 'session-picker-item-active' : 'session-picker-item'}
                onClick={() => {
                  setCurrentId(session.id);
                  setOpen(false);
                }}
                className={cn(
                  'block w-full text-left px-2 py-1 text-xs hover:bg-surface-3',
                  active && 'bg-primary-dim',
                )}
              >
                {session.name}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
