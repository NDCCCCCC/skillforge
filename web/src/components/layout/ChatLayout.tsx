// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatLayout — the chat-first app shell (Open Design redesign B9;
// replaces the three-panel IDE layout). Composition:
//
//   TopBar (56px,毛玻璃 B3)
//   ┌────────────────────────────────────────────────┐
//   │        chat column (flex-1, 840px cap)         │  chat-body
//   │  messages + composer, centered on FULL width   │
//   │ ▒session▒                          ▒detail▒    │  ← overlays
//   │ ▒panel▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒ │    (absolute,
//   └────────────────────────────────────────────────┘  z-30)
//   StatusBar (26px)
//
// B9 overlay semantics — the panels are ALWAYS-MOUNTED absolute
// overlays above the chat: toggling slides them in/out via CSS
// transform (data-open) so the centered chat column NEVER moves, and
// the panels' max-width safety cap keeps them out of the chat column
// on mid-size viewports. `inert` blocks focus/AT while closed. The
// detail CONTENT unmounts one slide-duration after closing (no
// closed-state re-renders during streaming); the session content
// stays mounted (cheap, keeps scroll position). Both panels persist
// width + open flag in localStorage (`sf.dp.*` / `sf.sp.*`) and share
// the delegated drag-to-resize listener (left: drag right → wider;
// right: drag left → wider).
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

  // no-transition guard — the hydration commit (stored open flags +
  // widths) and the removal of this class must NOT land in the same
  // paint, or every page refresh would replay the 300ms slide-in.
  // Two rAFs after mount, the values have painted; transitions are
  // safe to enable from then on.
  const [settled, setSettled] = useState(false);

  // Detail content lifecycle — mounted the instant the panel opens,
  // unmounted one slide-duration (~transition-base) AFTER it closes so
  // the exit animation paints real content while closed-state message
  // streaming never re-renders the DetailPanel subtree.
  const [detailMounted, setDetailMounted] = useState(() => readStoredOpen());

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

  // Enable transitions only after the hydration values have painted
  // (double rAF — see the `settled` declaration for why).
  useEffect(() => {
    let raf2 = 0;
    const raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(() => setSettled(true));
    });
    return () => {
      cancelAnimationFrame(raf1);
      cancelAnimationFrame(raf2);
    };
  }, []);

  // Delayed detail-content unmount (see `detailMounted` declaration).
  // Re-opening within the window cancels the pending unmount.
  useEffect(() => {
    if (detailPanelOpen) {
      setDetailMounted(true);
      return;
    }
    const t = setTimeout(() => setDetailMounted(false), 320);
    return () => clearTimeout(t);
  }, [detailPanelOpen]);

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
        {/* Left session panel — always-mounted overlay. data-open drives
            the CSS slide; inert blocks focus/clicks while closed (and
            during the exit slide). The scrim only mounts below 1280px
            (CSS gates its display) — on wide viewports the panel is a
            non-modal overlay and the chat stays interactive. */}
        {sessionPanelOpen && (
          <div
            data-testid="sp-scrim"
            className="sp-scrim"
            aria-hidden="true"
            onClick={() => useAppStore.getState().setSessionPanelOpen(false)}
          />
        )}
        <div
          data-testid="session-panel-wrap"
          className={cn('session-panel-wrap', !settled && 'no-transition')}
          data-open={sessionPanelOpen}
          inert={!sessionPanelOpen}
          style={{ width: sessionPanelWidth }}
        >
          <SessionDrawer
            open={sessionPanelOpen}
            onClose={() => useAppStore.getState().setSessionPanelOpen(false)}
          />
          <div
            data-testid="sp-resize-handle"
            className="resize-handle"
            aria-label="调整会话面板宽度"
          />
        </div>
        <div data-testid="chat-col" className="chat-col">
          {chat}
        </div>
        {/* Right detail panel — same overlay contract. The content
            ({detailMounted && detail}) unmounts after the exit slide;
            the wrap itself stays so the slide-out paints. */}
        {detailPanelOpen && (
          <div
            data-testid="dp-scrim"
            className="dp-scrim"
            aria-hidden="true"
            onClick={() => useAppStore.getState().setDetailPanelOpen(false)}
          />
        )}
        <div
          data-testid="detail-panel-wrap"
          className={cn('detail-panel-wrap', !settled && 'no-transition')}
          data-open={detailPanelOpen}
          inert={!detailPanelOpen}
          style={{ width: detailPanelWidth }}
        >
          {detailMounted && detail}
          <div
            data-testid="dp-resize-handle"
            className="resize-handle"
            aria-label="调整详情面板宽度"
          />
        </div>
      </div>
      <StatusBar />
    </div>
  );
}
