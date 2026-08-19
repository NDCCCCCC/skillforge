// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TopBar — the 56px topbar of the chat-first shell (Open Design
// redesign B2; formerly the 44px three-panel topbar).
//
// B2 composition (minimal migration — full Skill/Session pickers,
// knowledge + eval + theme entries land in B3):
//   - Brand logo + SkillForge label
//   - SkillManagerMenu trigger (installed Skills + 全部重载 / 新建
//     Skill（向导） CTAs — unchanged from Phase 5)
//   - Detail-panel toggle (replaces the two three-panel collapse
//     buttons)
import { Hexagon, PanelRight, PanelRightClose } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import SkillManagerMenu from './SkillManagerMenu';

export interface TopBarProps {
  /** Height in px (defaults to the CSS var --topbar-height). */
  height?: number;
}

export default function TopBar({ height }: TopBarProps) {
  const detailPanelOpen = useAppStore((s) => s.detailPanelOpen);
  const toggleDetailPanel = useAppStore((s) => s.toggleDetailPanel);

  return (
    <div
      className="topbar flex items-center justify-between border-b border-border bg-surface px-3"
      style={{ height: height ?? 'var(--topbar-height)' }}
      data-testid="app-topbar"
    >
      <div className="flex items-center gap-2">
        <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary-dim">
          <Hexagon className="h-4 w-4 text-primary" />
        </div>
        <div className="text-lg font-semibold tracking-tight text-text">SkillForge</div>
      </div>
      <div className="flex items-center gap-2">
        <SkillManagerMenu />
        <button
          type="button"
          data-testid="toggle-detail-panel"
          onClick={toggleDetailPanel}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label={detailPanelOpen ? '收起详情面板' : '展开详情面板'}
          aria-pressed={detailPanelOpen}
        >
          {detailPanelOpen ? (
            <PanelRightClose className="h-4 w-4" />
          ) : (
            <PanelRight className="h-4 w-4" />
          )}
        </button>
      </div>
    </div>
  );
}
