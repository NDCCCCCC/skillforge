// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TopBar — the 56px frosted-glass topbar of the chat-first shell
// (Open Design redesign B3: full composition). The deleted LeftSidebar
// surfaces are all re-homed here:
//   - Skill switcher → SkillPicker (live store.skills)
//   - Session list   → SessionPicker (store.sessions, 新建/删除 inline)
//   - Skill 管理/创建 → SkillManagerMenu (全部重载 / 新建 Skill 向导)
//   - 知识库文件树   → KnowledgeModal (LibraryBig trigger)
//   - Eval 入口      → BarChart3 button (pushState /eval + setRoute)
//   - 主题切换       → ThemeToggle (the B1 interim StatusBar button
//                       is retired)
//   - 详情面板开关   → retained from B2
import { useState } from 'react';
import { BarChart3, Hexagon, LibraryBig, PanelRight, PanelRightClose } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import SkillManagerMenu from './SkillManagerMenu';
import ThemeToggle from './ThemeToggle';
import KnowledgeModal from './KnowledgeModal';
import SkillPicker from '../chat/parts/SkillPicker';
import SessionPicker from '../chat/parts/SessionPicker';

export interface TopBarProps {
  /** Height in px (defaults to the CSS var --topbar-height). */
  height?: number;
}

export default function TopBar({ height }: TopBarProps) {
  const detailPanelOpen = useAppStore((s) => s.detailPanelOpen);
  const toggleDetailPanel = useAppStore((s) => s.toggleDetailPanel);
  const [knowledgeOpen, setKnowledgeOpen] = useState(false);

  // LeftSidebar's eval hop: real navigation when we're outside /eval,
  // plain store dispatch when already there (avoids duplicate history
  // entries when toggling between eval subroutes).
  function openEval() {
    if (window.location.pathname !== '/eval' && window.location.pathname !== '/eval/audit') {
      window.history.pushState({}, '', '/eval');
    }
    useAppStore.getState().setRoute('eval');
    useAppStore.getState().setEvalSubroute('dashboard');
  }

  return (
    <div
      className="topbar flex items-center justify-between gap-3 border-b border-border px-3"
      style={{ height: height ?? 'var(--topbar-height)' }}
      data-testid="app-topbar"
    >
      <div className="flex min-w-0 items-center gap-3">
        <div className="flex flex-none items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-primary-hover shadow-[0_0_16px_var(--color-glow-soft)]">
            <Hexagon className="h-4 w-4 text-white" />
          </div>
          <span className="hidden text-lg font-semibold tracking-tight text-text md:inline">
            SkillForge
          </span>
          <span className="hidden text-xs text-text-muted lg:inline">· 知识问答</span>
        </div>
        <div className="w-44 flex-none">
          <SkillPicker />
        </div>
        <div className="hidden w-44 flex-none md:block">
          <SessionPicker />
        </div>
      </div>
      <div className="flex flex-none items-center gap-1">
        <SkillManagerMenu />
        <button
          type="button"
          data-testid="topbar-knowledge"
          onClick={() => setKnowledgeOpen(true)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="知识库"
          title="知识库"
        >
          <LibraryBig className="h-4 w-4" />
        </button>
        <button
          type="button"
          data-testid="topbar-eval"
          onClick={openEval}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="评估"
          title="评估"
        >
          <BarChart3 className="h-4 w-4" />
        </button>
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
        <ThemeToggle />
      </div>
      <KnowledgeModal open={knowledgeOpen} onClose={() => setKnowledgeOpen(false)} />
    </div>
  );
}
