// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChatLayout — the chat-first app shell (Open Design redesign B2;
// replaces the three-panel IDE layout). Composition:
//
//   TopBar (56px,毛玻璃 B3)
//   ┌────────────────────────────┬──────────────┐
//   │ chat column (flex-1)       │ detail panel │  chat-body
//   │  messages + composer       │ (collapsible │
//   │                            │  + draggable)│
//   └────────────────────────────┴──────────────┘
//   StatusBar (26px)
//
// The left sidebar is GONE — sessions/skills/knowledge move to the
// TopBar + modals (B3). The detail panel (former center panel) is
// closed by default, opens via toggleDetailPanel / verdict chips, and
// keeps its width in localStorage (`sf.dp.open` / `sf.dp.w`, clamp
// 320–520). Drag-to-resize follows the ThreePanel pattern: one
// delegated mousedown listener, inline width during drag for the
// smoothest response, store + localStorage write on mouseup.
import { useEffect, useState, type ReactNode } from 'react';
import { useAppStore } from '../../store/app-store';
import { cn } from '../../lib/cn';
import StatusBar from '../status/StatusBar';
import TopBar from './TopBar';

const LS_KEY_OPEN = 'sf.dp.open';
const LS_KEY_WIDTH = 'sf.dp.w';
const MIN_WIDTH = 320;
const MAX_WIDTH = 520;
const DEFAULT_WIDTH = 380;

function clampWidth(value: number): number {
  if (Number.isNaN(value)) return DEFAULT_WIDTH;
  return Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, value));
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

export interface ChatLayoutProps {
  chat: ReactNode;
  detail: ReactNode;
}

export default function ChatLayout({ chat, detail }: ChatLayoutProps) {
  const detailPanelOpen = useAppStore((s) => s.detailPanelOpen);
  const detailPanelWidth = useAppStore((s) => s.detailPanelWidth);
  const setDetailPanelWidth = useAppStore((s) => s.setDetailPanelWidth);

  const [hydrated, setHydrated] = useState(false);

  // Hydrate from localStorage once on mount (width + open flag). The
  // fallback is the CURRENT store width so a missing LS entry never
  // clobbers a non-default store value (ThreePanel semantics).
  useEffect(() => {
    const storedWidth = readStoredWidth(detailPanelWidth);
    if (storedWidth !== detailPanelWidth) setDetailPanelWidth(storedWidth);
    if (readStoredOpen()) useAppStore.getState().setDetailPanelOpen(true);
    setHydrated(true);
    // Intentionally run only on mount; the localStorage snapshot seeds
    // the store on first render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Persist the open flag AFTER hydration so the first-render write
  // cannot clobber the stored value we just read.
  useEffect(() => {
    if (!hydrated) return;
    try {
      window.localStorage.setItem(LS_KEY_OPEN, detailPanelOpen ? '1' : '0');
    } catch {
      // storage unavailable — session-only open state
    }
  }, [detailPanelOpen, hydrated]);

  // Drag-to-resize: mousedown delegated from the handle. Inline width
  // during drag (Zustand re-render lags React batching); store + LS
  // write on mouseup mirrors the ThreePanel pattern.
  useEffect(() => {
    function onMouseDown(event: MouseEvent) {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      const handle = target.closest('[data-testid="dp-resize-handle"]') as HTMLElement | null;
      if (!handle) return;
      const panelEl = document.querySelector('[data-testid="detail-panel-wrap"]') as HTMLElement | null;
      if (!panelEl) return;

      event.preventDefault();
      const startX = event.clientX;
      const startWidth = panelEl.getBoundingClientRect().width;
      let finalWidth = startWidth;

      function onMove(e: MouseEvent) {
        const dx = e.clientX - startX;
        // Right-side panel: drag left → wider.
        finalWidth = clampWidth(startWidth - dx);
        panelEl.style.width = `${finalWidth}px`;
      }

      function onUp() {
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        document.body.removeAttribute('data-dragging');
        setDetailPanelWidth(finalWidth);
        try {
          window.localStorage.setItem(LS_KEY_WIDTH, String(finalWidth));
        } catch {
          // storage unavailable — session-only width
        }
      }

      document.body.setAttribute('data-dragging', 'true');
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    }

    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [setDetailPanelWidth]);

  return (
    <div className="app-shell" data-testid="app-shell">
      <TopBar />
      <div data-testid="chat-body" className="chat-body">
        <div data-testid="chat-col" className="chat-col">
          {chat}
        </div>
        {detailPanelOpen && (
          <>
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
