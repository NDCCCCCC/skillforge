// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TopBar — the 44px topbar of the ThreePanel shell. Phase 4 inlined
// this inside ThreePanel.tsx; Phase 5 extracts it so the SC-2 skill-
// manager menu + the SC-3 wizard entry point live in this file.
//
// Composition:
//   - Brand logo + SkillForge label (Phase 4 inherited)
//   - SkillManagerMenu trigger (Phase 5 NEW — replaces the Phase-4
//     chevron stub which is no-op). Opens a Command-driven dropdown
//     with installed Skills + 全部重载 / 新建 Skill（向导） CTAs.
//   - Left + right sidebar collapse buttons (Phase 4 inherited).
import { Hexagon, PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import SkillManagerMenu from './SkillManagerMenu';

export interface TopBarProps {
  /** Height in px (defaults to the CSS var --topbar-height). */
  height?: number;
}

export default function TopBar({ height }: TopBarProps) {
  const sidebarLeft = useAppStore((s) => s.sidebarLeft);
  const sidebarRight = useAppStore((s) => s.sidebarRight);
  const toggleSidebar = useAppStore((s) => s.toggleSidebar);

  return (
    <div
      className="flex items-center justify-between px-3 border-b border-border bg-surface"
      style={{ height: height ?? 'var(--topbar-height)' }}
      data-testid="app-topbar"
    >
      <div className="flex items-center gap-2">
        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary-dim">
          <Hexagon className="h-4 w-4 text-primary" />
        </div>
        <div className="text-lg font-semibold text-text">SkillForge</div>
      </div>
      <div className="flex items-center gap-2">
        {/* Phase 5 — replaces the Phase-4 chevron stub with the real
            SkillManagerMenu trigger. The chevron's no-op was tracked
            as a Phase-4-deferred item per 05-UI-SPEC. */}
        <SkillManagerMenu />
        <button
          type="button"
          data-testid="collapse-left"
          onClick={() => toggleSidebar('left')}
          className="inline-flex items-center justify-center h-8 w-8 rounded-md text-text-muted hover:text-text hover:bg-surface-2 transition-colors"
          aria-label={sidebarLeft ? 'Collapse left panel' : 'Expand left panel'}
        >
          {sidebarLeft ? (
            <PanelLeftClose className="h-4 w-4" />
          ) : (
            <PanelLeftOpen className="h-4 w-4" />
          )}
        </button>
        <button
          type="button"
          data-testid="collapse-right"
          onClick={() => toggleSidebar('right')}
          className="inline-flex items-center justify-center h-8 w-8 rounded-md text-text-muted hover:text-text hover:bg-surface-2 transition-colors"
          aria-label={sidebarRight ? 'Collapse right panel' : 'Expand right panel'}
        >
          {sidebarRight ? (
            <PanelRightClose className="h-4 w-4" />
          ) : (
            <PanelRightOpen className="h-4 w-4" />
          )}
        </button>
      </div>
    </div>
  );
}
