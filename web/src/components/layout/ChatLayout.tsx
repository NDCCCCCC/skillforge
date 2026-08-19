// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatLayout — the chat-first app shell (Open Design redesign B2;
// replaces the three-panel IDE layout). Composition:
//
//   TopBar (56px,毛玻璃 B3)
//   ┌──────────┬────────────────────────────┬──────────────┐
//   │ session  │ chat column (flex-1)       │ detail panel │  chat-body
//   │ panel    │  messages + composer       │ (collapsible │
//   │ (inline, │                            │  + draggable)│
//   │ left)    │                            │              │
//   └──────────┴────────────────────────────┴──────────────┘
//   StatusBar (26px)
//
// Session-management phase — the LEFT session panel mirrors the right
// DetailPanel pattern exactly: an INLINE column ≥1280px (chat
// compresses but stays fully usable — the user's ask), a fixed
// overlay + scrim below that. Both panels persist width + open flag
// in localStorage (`sf.dp.*` / `sf.sp.*`) and share the delegated
// drag-to-resize listener (left: drag right → wider; right: drag
// left → wider).
import { useEffect, useState, type ReactNode } from 'react';
import { useAppStore } from '../../store/app-store';
import { cn } from '../../lib/cn';
import StatusBar from '../status/StatusBar';
import TopBar from './TopBar';
import SessionDrawer from './SessionDrawer';

const LS_KEY_OPEN = 'sf.dp.open';
const LS_KEY_WIDTH = 'sf.dp.w';
const MIN_WIDTH = 320;
const MAX_WIDTH = 520;
const DEFAULT_WIDTH = 380;

// Session panel — narrower than the detail panel (it's a roster, not
// a document surface).
const SP_LS_KEY_OPEN = 'sf.sp.open';
const SP_LS_KEY_WIDTH = 'sf.sp.w';
const SP_MIN_WIDTH = 220;
const SP_MAX_WIDTH = 360;
const SP_DEFAULT_WIDTH = 280;

function clampWidth(value: number): number {
  if (Number.isNaN(value)) return DEFAULT_WIDTH;
  return Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, value));
}

function clampSessionWidth(value: number): number {
  if (Number.isNaN(value)) return SP_DEFAULT_WIDTH;
  return Math.max(SP_MIN_WIDTH, Math.min(SP_MAX_WIDTH, value));
}

function readStoredWidth(fallback: number): number {
  if (typeof window === 'undefined') return fallback;
  const raw = window.localStorage.getItem(LS_KEY_WIDTH);
  if (!raw) return fallback;
  return clampWidth(parseInt(raw, 10));
}

function readStoredOpen(): boolean {
  if (typeof window === 'undefined') return false;
  return window.localStorage.getItem(LS_KEY_OPEN) === '1';
}

function readStoredSessionWidth(fallback: number): number {
  if (typeof window === 'undefined') return fallback;
  const raw = window.localStorage.getItem(SP_LS_KEY_WIDTH);
  if (!raw) return fallback;
  return clampSessionWidth(parseInt(raw, 10));
}

function readStoredSessionOpen(): boolean {
  if (typeof window === 'undefined') return false;
  return window.localStorage.getItem(SP_LS_KEY_OPEN) === '1';
}

export interface ChatLayoutProps {
  chat: ReactNode;
  detail: ReactNode;
}

export default function ChatLayout({ chat, detail }: ChatLayoutProps) {
  const detailPanelOpen = useAppStore((s) => s.detailPanelOpen);
  const detailPanelWidth = useAppStore((s) => s.detailPanelWidth);
  const setDetailPanelWidth = useAppStore((s) => s.setDetailPanelWidth);
  const sessionPanelOpen = useAppStore((s) => s.sessionPanelOpen);
  const sessionPanelWidth = useAppStore((s) => s.sessionPanelWidth);
  const setSessionPanelWidth = useAppStore((s) => s.setSessionPanelWidth);

  const [hydrated, setHydrated] = useState(false);

  // Hydrate from localStorage once on mount (width + open flag). The
  // fallback is the CURRENT store width so a missing LS entry never
  // clobbers a non-default store value (ThreePanel semantics).
  useEffect(() => {
    const storedWidth = readStoredWidth(detailPanelWidth);
    if (storedWidth !== detailPanelWidth) setDetailPanelWidth(storedWidth);
    if (readStoredOpen()) useAppStore.getState().setDetailPanelOpen(true);
    const storedSpWidth = readStoredSessionWidth(sessionPanelWidth);
    if (storedSpWidth !== sessionPanelWidth) setSessionPanelWidth(storedSpWidth);
    if (readStoredSessionOpen()) useAppStore.getState().setSessionPanelOpen(true);
    setHydrated(true);
    // Intentionally run only on mount; the localStorage snapshot seeds
    // the store on first render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Persist the open flags AFTER hydration so the first-render write
  // cannot clobber the stored value we just read.
  useEffect(() => {
    if (!hydrated) return;
    try {
      window.localStorage.setItem(LS_KEY_OPEN, detailPanelOpen ? '1' : '0');
      window.localStorage.setItem(SP_LS_KEY_OPEN, sessionPanelOpen ? '1' : '0');
    } catch {
      // storage unavailable — session-only open state
    }
  }, [detailPanelOpen, sessionPanelOpen, hydrated]);

  // Drag-to-resize: mousedown delegated from either handle. Inline
  // width during drag (Zustand re-render lags React batching); store +
  // LS write on mouseup mirrors the ThreePanel pattern.
  useEffect(() => {
    function onMouseDown(event: MouseEvent) {
      const target = event.target as HTMLElement | null;
      if (!target) return;

      // Right-side detail panel: drag left → wider.
      const dpHandle = target.closest('[data-testid="dp-resize-handle"]') as HTMLElement | null;
      if (dpHandle) {
        const panelEl = document.querySelector('[data-testid="detail-panel-wrap"]') as HTMLElement | null;
        if (!panelEl) return;
        startDrag(event, panelEl, clampWidth, (w) => {
          setDetailPanelWidth(w);
          try {
            window.localStorage.setItem(LS_KEY_WIDTH, String(w));
          } catch {
            /* storage unavailable — session-only width */
          }
        }, 'right');
        return;
      }

      // Left-side session panel: drag right → wider.
      const spHandle = target.closest('[data-testid="sp-resize-handle"]') as HTMLElement | null;
      if (spHandle) {
        const panelEl = document.querySelector('[data-testid="session-panel-wrap"]') as HTMLElement | null;
        if (!panelEl) return;
        startDrag(event, panelEl, clampSessionWidth, (w) => {
          setSessionPanelWidth(w);
          try {
            window.localStorage.setItem(SP_LS_KEY_WIDTH, String(w));
          } catch {
            /* storage unavailable — session-only width */
          }
        }, 'left');
      }
    }

    function startDrag(
      event: MouseEvent,
      panelEl: HTMLElement,
      clamp: (v: number) => number,
      commit: (width: number) => void,
      side: 'left' | 'right',
    ) {
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = panelEl.getBoundingClientRect().width;
      let finalWidth = startWidth;

      function onMove(e: MouseEvent) {
        const dx = e.clientX - startX;
        finalWidth = clamp(side === 'right' ? startWidth - dx : startWidth + dx);
        panelEl.style.width = `${finalWidth}px`;
      }

      function onUp() {
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        document.body.removeAttribute('data-dragging');
        commit(finalWidth);
      }

      document.body.setAttribute('data-dragging', 'true');
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    }

    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [setDetailPanelWidth, setSessionPanelWidth]);

  return (
    <div className="app-shell" data-testid="app-shell">
      <TopBar />
      <div data-testid="chat-body" className="chat-body">
        {sessionPanelOpen && (
          <>
            {/* <1280px scrim under the fixed-overlay session panel (CSS
                hides it ≥1280). Click to dismiss mirrors dp-scrim. */}
            <div
              data-testid="sp-scrim"
              className="sp-scrim"
              aria-hidden="true"
              onClick={() => useAppStore.getState().setSessionPanelOpen(false)}
            />
            <div
              data-testid="session-panel-wrap"
              className={cn('session-panel-wrap', !hydrated && 'no-transition')}
              style={{ width: sessionPanelWidth }}
            >
              <SessionDrawer
                open
                onClose={() => useAppStore.getState().setSessionPanelOpen(false)}
              />
            </div>
            <div
              data-testid="sp-resize-handle"
              className="resize-handle"
              aria-label="调整会话面板宽度"
            />
          </>
        )}
        <div data-testid="chat-col" className="chat-col">
          {chat}
        </div>
        {detailPanelOpen && (
          <>
            {/* B7 — scrim under the panel on <1280px viewports where the
                panel is a fixed overlay (CSS hides it ≥1280). Click to
                dismiss mirrors the KnowledgeModal scrim contract. */}
            <div
              data-testid="dp-scrim"
              className="dp-scrim"
              aria-hidden="true"
              onClick={() => useAppStore.getState().setDetailPanelOpen(false)}
            />
            <div
              data-testid="dp-resize-handle"
              className="resize-handle"
              aria-label="调整详情面板宽度"
            />
            <div
              data-testid="detail-panel-wrap"
              className={cn('detail-panel-wrap', !hydrated && 'no-transition')}
              style={{ width: detailPanelWidth }}
            >
              {detail}
            </div>
          </>
        )}
      </div>
      <StatusBar />
    </div>
  );
}
