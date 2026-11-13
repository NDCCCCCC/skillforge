// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TopBar — the 56px frosted-glass topbar of the chat-first shell
// (Open Design redesign B3: full composition). The deleted LeftSidebar
// surfaces are all re-homed here:
//   - Skill switcher → SkillPicker (live store.skills)
//   - Session list   → SessionPicker (store.sessions, 新建/删除 inline)
//                     + 会话列表 panel toggle (moved to the LEFT zone,
//                       261113-jau: the panel it opens is the left
//                       session-panel-wrap — entry and panel now align)
//   - 新建会话        → Plus button in the right zone (261113-jau:
//                       direct createSession, replaces the old right-
//                       zone session-list entry)
//   - Skill 管理/创建 → SkillManagerMenu (全部重载 / 新建 Skill 向导)
//   - 知识库文件树   → KnowledgeModal (LibraryBig trigger)
//   - Eval 入口      → BarChart3 button (pushState /eval + setRoute)
//   - 主题切换       → ThemeToggle (the B1 interim StatusBar button
//                       is retired)
//   - 详情面板开关   → retained from B2
import { useState } from 'react';
import { BarChart3, Hexagon, LibraryBig, MessagesSquare, PanelRight, PanelRightClose, Plus } from 'lucide-react';
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
  const sessionPanelOpen = useAppStore((s) => s.sessionPanelOpen);
  const setSessionPanelOpen = useAppStore((s) => s.setSessionPanelOpen);
  const createSession = useAppStore((s) => s.createSession);
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
        {/* Session panel entry (261113-jau) — moved from the right zone
            into the LEFT zone so the entry sits beside SessionPicker,
            aligned with the left-anchored session-panel-wrap it opens.
            ALWAYS visible (the SessionPicker dropdown is hidden below
            md; without this button mobile has no session surface). */}
        <button
          type="button"
          data-testid="topbar-sessions"
          onClick={() => setSessionPanelOpen(!sessionPanelOpen)}
          aria-pressed={sessionPanelOpen}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="会话列表"
          title="会话列表"
          data-tip="会话列表"
        >
          <MessagesSquare className="h-4 w-4" />
        </button>
      </div>
      <div className="flex flex-none items-center gap-1">
        <SkillManagerMenu />
        {/* 261113-jau — direct 新建会话: replaces the old right-zone
            session-list entry (the panel toggle now lives in the left
            zone, next to SessionPicker). One click mints + activates a
            session via the store action. */}
        <button
          type="button"
          data-testid="topbar-new-session"
          onClick={() => createSession()}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="新建会话"
          title="新建会话"
          data-tip="新建会话"
        >
          <Plus className="h-4 w-4" />
        </button>
        <button
          type="button"
          data-testid="topbar-knowledge"
          onClick={() => setKnowledgeOpen(true)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="知识库"
          title="知识库"
          data-tip="知识库"
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
          data-tip="评估"
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
          data-tip={detailPanelOpen ? '收起详情面板' : '展开详情面板'}
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
