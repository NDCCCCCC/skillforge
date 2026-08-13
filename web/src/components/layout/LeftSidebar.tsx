// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// LeftSidebar — Plan 05-07 (D5-IA-02/03/04) 4-section rewrite.
//
// Renders 4 stacked labeled sections top-to-bottom:
//   1. 新建会话 button — calls createSession() (D5-IA-02/03).
//   2. 会话 block — list of useAppStore.sessions (D5-IA-03).
//      UI-local Zustand; messages are owned by useChat per id; lost on
//      refresh (no backend ListSessions / /api/sessions added).
//   3. 文件 block — preserved SOURCES tier list (mocked; real file
//      tree is a separate effort per CONTEXT deferred items).
//   4. 技能 block — list of useAppStore.skills with status badges
//      (已加载 / 加载失败（旧版可用） per UI-SPEC §Copywriting
//      Contract Phase-4-deferred items) per D5-IA-04 division of
//      labor — Skill browse + status live here, not in the topbar
//      SkillManagerMenu.
//
// Threat model: zero dangerouslySetInnerHTML on backend-derived skill
// fields (T-5-IA-01); no client-side path construction from
// currentSkillId (T-5-IA-02).
import { CheckCircle2, FileWarning, Folder, FolderOpen, MessageCircle, Plus, Wrench } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';

const SOURCES = [
  { tier: '权威', path: 'testdata/docs/authority/' },
  { tier: '历史', path: 'testdata/docs/historical/' },
  { tier: '个人', path: 'testdata/docs/personal/' },
  { tier: '草稿', path: 'testdata/docs/draft/' },
];

export default function LeftSidebar() {
  // Plan 05-07 — subscribe to each slice individually so re-renders
  // are scoped (Zustand re-renders only on slice change).
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const skills = useAppStore((s) => s.skills);
  const currentSkillId = useAppStore((s) => s.currentSkillId);

  function handleNewSession() {
    useAppStore.getState().createSession();
  }

  function handleSelectSession(id: string) {
    useAppStore.getState().setActiveSession(id);
  }

  function handleSelectSkill(id: string) {
    useAppStore.getState().setCurrentSkill(id);
  }

  return (
    <div
      data-testid="left-sidebar"
      className="h-full bg-surface border-r border-border overflow-y-auto"
    >
      {/* 1. 新建会话 button */}
      <div className="px-3 pt-3 pb-2">
        <button
          type="button"
          data-testid="left-sidebar-new-session"
          onClick={handleNewSession}
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>新建会话</span>
        </button>
      </div>

      {/* 2. 会话 block */}
      <section>
        <div className="flex items-center gap-1.5 px-3 pt-2 pb-1 text-text-muted">
          <MessageCircle className="h-3.5 w-3.5 shrink-0" />
          <h4 className="text-xs font-semibold uppercase tracking-wider">会话 · SESSIONS</h4>
        </div>
        <ul className="px-2 pb-3 space-y-0.5">
          {sessions.length === 0 ? (
            <li className="px-2 py-1 text-xs text-text-muted">暂无会话</li>
          ) : (
            sessions.map((s) => {
              const active = s.id === activeSessionId;
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    data-testid="left-sidebar-session"
                    data-session-id={s.id}
                    onClick={() => handleSelectSession(s.id)}
                    className={cn(
                      'block w-full text-left rounded-md px-2 py-1.5 text-xs text-text hover:bg-surface-2 transition-colors',
                      active && 'bg-primary-dim border-l-2 border-l-primary',
                    )}
                  >
                    <span className="truncate">{s.name}</span>
                  </button>
                </li>
              );
            })
          )}
        </ul>
      </section>

      {/* 3. 文件 block — preserved SOURCES tier list */}
      <section>
        <div className="flex items-center gap-1.5 px-3 pt-2 pb-1 text-text-muted">
          <FolderOpen className="h-3.5 w-3.5 shrink-0" />
          <h4 className="text-xs font-semibold uppercase tracking-wider">文件 · FILES</h4>
        </div>
        <ul className="px-2 pb-3 space-y-0.5">
          {SOURCES.map((src) => (
            <li
              key={src.path}
              data-testid="file-item"
              className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-surface-2 transition-colors cursor-default"
            >
              <Folder className="h-3.5 w-3.5 shrink-0 text-text-muted" />
              <div className="flex flex-col min-w-0">
                <span className="text-xs font-medium text-text">{src.tier}</span>
                <span className="text-xs font-mono text-text-dim truncate">{src.path}</span>
              </div>
            </li>
          ))}
        </ul>
      </section>

      {/* 4. 技能 block — D5-IA-04 */}
      <section>
        <div className="flex items-center gap-1.5 px-3 pt-2 pb-1 text-text-muted">
          <Wrench className="h-3.5 w-3.5 shrink-0" />
          <h4 className="text-xs font-semibold uppercase tracking-wider">技能 · SKILLS</h4>
        </div>
        <ul className="px-2 pb-3 space-y-0.5">
          {skills.length === 0 ? (
            <li
              data-testid="left-sidebar-skill-empty"
              className="px-2 py-1 text-xs text-text-muted"
            >
              尚未安装任何 Skill
            </li>
          ) : (
            skills.map((skill) => {
              const failed = skill.status === 'failed';
              const active = skill.id === currentSkillId;
              return (
                <li key={skill.id}>
                  <button
                    type="button"
                    data-testid="left-sidebar-skill"
                    data-skill-id={skill.id}
                    title={failed && skill.error ? `加载失败：${skill.error}` : undefined}
                    onClick={() => handleSelectSkill(skill.id)}
                    className={cn(
                      'flex w-full items-center justify-between gap-2 rounded-md px-2 py-1 text-xs text-left hover:bg-surface-2 transition-colors',
                      active && 'bg-primary-dim border-l-2 border-l-primary',
                    )}
                  >
                    <span className="truncate text-text">{skill.name}</span>
                    <span
                      data-testid={failed ? 'left-sidebar-skill-failed' : 'left-sidebar-skill-ok'}
                      className={cn(
                        'inline-flex items-center gap-1 text-xs',
                        failed ? 'text-status-non-compliant' : 'text-status-compliant',
                      )}
                    >
                      {failed ? (
                        <>
                          <FileWarning className="h-3 w-3" />
                          <span>加载失败（旧版可用）</span>
                        </>
                      ) : (
                        <>
                          <CheckCircle2 className="h-3 w-3" />
                          <span>已加载</span>
                        </>
                      )}
                    </span>
                  </button>
                </li>
              );
            })
          )}
        </ul>
      </section>
    </div>
  );
}