// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// LeftSidebar — Plan 05-07 (D5-IA-02/03/04) 4-section rewrite
//              + Plan 05-08 (WR-01, WR-04, IN-01, IN-04) opportunistic
//                cleanup — stripFsPath in failed-skill title, multi-line
//                error flattening, × delete-session button per row,
//                skill-service-offline banner above the 技能 block.
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
// currentSkillId (T-5-IA-02). Failed-skill title attributes are
// passed through stripFsPath + multi-line flattening + 160-char
// truncation (T-5-IA-04 closure — WR-01/04).
import { CheckCircle2, FileWarning, Folder, FolderOpen, MessageCircle, Plus, Wrench } from 'lucide-react';
import { cn } from '../../lib/cn';
import { stripFsPath } from '../../lib/stripFsPath';
import { useAppStore } from '../../store/app-store';

const SOURCES = [
  { tier: '权威', path: 'testdata/docs/authority/' },
  { tier: '历史', path: 'testdata/docs/historical/' },
  { tier: '个人', path: 'testdata/docs/personal/' },
  { tier: '草稿', path: 'testdata/docs/draft/' },
];

/** Plan 05-08 (WR-04) — flatten multi-line error strings into a single
 * truncated line for use in title attributes. Browser tooltip rendering
 * is OS-dependent; HTML-escaped text + 160 char truncation keeps the
 * tooltip predictable across platforms. */
function flattenError(s: string): string {
  const stripped = stripFsPath(s) ?? s;
  const flat = stripped.replace(/\r?\n/g, ' ');
  return flat.length > 160 ? `${flat.slice(0, 160)}…` : flat;
}

export default function LeftSidebar() {
  // Plan 05-07 — subscribe to each slice individually so re-renders
  // are scoped (Zustand re-renders only on slice change).
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const skills = useAppStore((s) => s.skills);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  // Plan 05-08 (IN-04) — subscribe to the loud-failure flag set by
  // useSkillReload after 3 consecutive non-OK /api/skills responses.
  const skillServiceOffline = useAppStore((s) => s.skillServiceOffline);

  function handleNewSession() {
    useAppStore.getState().createSession();
  }

  function handleSelectSession(id: string) {
    useAppStore.getState().setActiveSession(id);
  }

  function handleSelectSkill(id: string) {
    useAppStore.getState().setCurrentSkill(id);
  }

  // Plan 05-08 (IN-01) — deleteSession is now reachable via a small ×
  // button on each session row. event.stopPropagation prevents the
  // parent row's setActiveSession from also firing on the same click.
  function handleDeleteSession(id: string, e: React.MouseEvent) {
    e.stopPropagation();
    useAppStore.getState().deleteSession(id);
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
                <li key={s.id} className="relative group">
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
                  <button
                    type="button"
                    data-testid="left-sidebar-session-delete"
                    data-session-id={s.id}
                    onClick={(e) => handleDeleteSession(s.id, e)}
                    aria-label={`删除会话 ${s.name}`}
                    className="absolute right-1 top-1/2 -translate-y-1/2 inline-flex h-5 w-5 items-center justify-center rounded-md text-text-muted opacity-0 group-hover:opacity-100 hover:bg-surface-3 hover:text-text"
                  >
                    ×
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
        {/* Plan 05-08 (IN-04) — loud-failure banner. Visible above the
            技能 block when useSkillReload has observed 3+ consecutive
            non-OK /api/skills responses. */}
        {skillServiceOffline && (
          <div
            data-testid="skill-service-offline-banner"
            className="mx-2 mb-2 rounded-md border border-status-non-compliant/40 bg-surface-2 px-2 py-1 text-xs text-status-non-compliant"
          >
            无法连接到 Skill 服务 — 显示的内容可能已过时
          </div>
        )}
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
                    title={failed && skill.error ? `加载失败：${flattenError(skill.error)}` : undefined}
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