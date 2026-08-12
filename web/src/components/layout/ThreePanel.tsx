// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ThreePanel — the three-column draggable layout (260/flex/420) with
// localStorage persistence + collapse toggles per sketch-findings
// `layout-three-panel.md`.
//
// Plan 04-03 → 05-03 refactor: the topbar and statusbar were inlined
// here in Phase 4. Phase 5 extracts them into TopBar.tsx + StatusBar.tsx
// so the SC-2 skill-manager menu + SC-2 in-flight pinned-version
// indicator live in their own files (per 05-UI-SPEC §Component
// Contracts). The grid layout + drag-to-resize + collapse behavior is
// unchanged.
//
// Topbar (44px) → grid (1fr) → statusbar (26px) = the .app-shell CSS
// class. The grid rows hold the topbar (now TopBar), the three panels
// + resize handles, and the statusbar (now StatusBar).
import { useEffect, useState, type ReactNode } from 'react';
import { useAppStore } from '../../store/app-store';
import { cn } from '../../lib/cn';
import StatusBar from '../status/StatusBar';
import TopBar from './TopBar';

const LS_KEY_LEFT = 'sf-left-width';
const LS_KEY_RIGHT = 'sf-right-width';
const MIN_WIDTH = 180;
const MAX_WIDTH = 600;

function clampWidth(value: number): number {
  if (Number.isNaN(value)) return 260;
  return Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, value));
}

function readStoredWidth(key: string, fallback: number): number {
  if (typeof window === 'undefined') return fallback;
  const raw = window.localStorage.getItem(key);
  if (!raw) return fallback;
  const parsed = parseInt(raw, 10);
  return clampWidth(parsed);
}

export interface ThreePanelProps {
  left: ReactNode;
  center: ReactNode;
  right: ReactNode;
}

export default function ThreePanel({ left, center, right }: ThreePanelProps) {
  const leftWidth = useAppStore((s) => s.leftWidth);
  const rightWidth = useAppStore((s) => s.rightWidth);
  const sidebarLeft = useAppStore((s) => s.sidebarLeft);
  const sidebarRight = useAppStore((s) => s.sidebarRight);
  const setPanelWidth = useAppStore((s) => s.setPanelWidth);

  const [hydrated, setHydrated] = useState(false);
  const [dragStyle, setDragStyle] = useState<{ left: number; right: number } | null>(null);

  // Hydrate from localStorage once on mount.
  useEffect(() => {
    const storedLeft = readStoredWidth(LS_KEY_LEFT, leftWidth);
    const storedRight = readStoredWidth(LS_KEY_RIGHT, rightWidth);
    if (storedLeft !== leftWidth) setPanelWidth('left', storedLeft);
    if (storedRight !== rightWidth) setPanelWidth('right', storedRight);
    setHydrated(true);
    // Intentionally run only on mount; we want the localStorage
    // snapshot to seed the store on first render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Drag-to-resize: one mousedown listener delegated from the resize
  // handles. On drag, update the inline grid template + persist via
  // Zustand. On mouseup, write to localStorage.
  useEffect(() => {
    function onMouseDown(event: MouseEvent) {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      const handle = target.closest('[data-testid="resize-handle-left"], [data-testid="resize-handle-right"]') as HTMLElement | null;
      if (!handle) return;
      const side = handle.dataset.side === 'left' ? 'left' : 'right';
      const mainEl = document.querySelector('[data-testid="app-main"]') as HTMLElement | null;
      const panelEl = document.querySelector(
        side === 'left' ? '[data-testid="panel-left"]' : '[data-testid="panel-right"]',
      ) as HTMLElement | null;
      if (!mainEl || !panelEl) return;

      event.preventDefault();
      const startX = event.clientX;
      const startWidth = panelEl.getBoundingClientRect().width;

      function onMove(e: MouseEvent) {
        const dx = e.clientX - startX;
        const newWidth = side === 'left' ? startWidth + dx : startWidth - dx;
        const clamped = clampWidth(newWidth);
        // Update inline grid template directly for the smoothest
        // drag response (Zustand re-render is delayed by React's
        // batch scheduling). The grid is 5 tracks
        // [left, handle, center, handle, right]; the right panel lives
        // at cols[4] (not cols[2], which is the center track).
        const cols = getComputedStyle(mainEl!).gridTemplateColumns.split(' ');
        const leftCol = side === 'left' ? `${clamped}px` : cols[0];
        const rightCol = side === 'right' ? `${clamped}px` : cols[4];
        mainEl!.style.gridTemplateColumns = `${leftCol} 4px 1fr 4px ${rightCol}`;
        setDragStyle({ left: clamped, right: clamped });
      }

      function onUp() {
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        document.body.removeAttribute('data-dragging');
        // Persist the final width via Zustand + localStorage.
        const finalWidth = (dragStyle && (side === 'left' ? dragStyle.left : dragStyle.right)) ?? startWidth;
        const clamped = clampWidth(finalWidth);
        setPanelWidth(side, clamped);
        if (typeof window !== 'undefined') {
          window.localStorage.setItem(side === 'left' ? LS_KEY_LEFT : LS_KEY_RIGHT, String(clamped));
        }
        // After drag-end, drop the inline override so the store-driven
        // grid template takes over for the next render.
        setDragStyle(null);
      }

      document.body.setAttribute('data-dragging', 'true');
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    }

    document.addEventListener('mousedown', onMouseDown);
    return () => {
      document.removeEventListener('mousedown', onMouseDown);
    };
  }, [dragStyle, setPanelWidth]);

  // Compute the grid template. 5 tracks: [left, handle, center, handle,
  // right]. The 4px handle tracks match `.resize-handle` width so the
  // handles occupy their own column; collapsed sides collapse to 0px.
  const leftCol = !sidebarLeft ? '0px' : `${leftWidth}px`;
  const rightCol = !sidebarRight ? '0px' : `${rightWidth}px`;
  const gridTemplateColumns = `${leftCol} 4px 1fr 4px ${rightCol}`;

  return (
    <div className="app-shell" data-testid="app-shell">
      {/* Topbar — Phase 5 extracted to TopBar.tsx for the SkillManagerMenu trigger */}
      <TopBar />

      {/* Main grid */}
      <div
        data-testid="app-main"
        className={cn('app-main', hydrated && 'app-main-hydrated')}
        style={{ gridTemplateColumns }}
      >
        <div data-testid="panel-left" className="overflow-hidden">
          {left}
        </div>
        <div
          data-testid="resize-handle-left"
          data-side="left"
          className="resize-handle"
          aria-label="Resize left panel"
        />
        <div data-testid="panel-center" className="overflow-hidden">
          {center}
        </div>
        <div
          data-testid="resize-handle-right"
          data-side="right"
          className="resize-handle"
          aria-label="Resize right panel"
        />
        <div data-testid="panel-right" className="overflow-hidden">
          {right}
        </div>
      </div>

      {/* Statusbar — Phase 5 extracted to StatusBar.tsx for the SC-2 pinned-version indicator */}
      <StatusBar />
    </div>
  );
}
