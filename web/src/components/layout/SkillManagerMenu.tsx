// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillManagerMenu — Plan 05-03 / SC-2 + SC-3 topbar dropdown.
//
// Phase 4 had a no-op chevron stub in the topbar; Phase 5 replaces it
// with a real command-driven dropdown listing installed Skills with
// per-skill status (loaded / reload-failed) and CTAs:
//
//   - 全部重载   — triggers a fsnotify rescan (POST /api/skills/reload)
//   - 新建 Skill（向导） — opens the SC-3 wizard (Plan 05-04)
//
// The shadcn `Command` primitive is NOT yet initialized (shadcn init
// runs as a Wave-1 task per UI-SPEC §Registry Safety); we hand-roll a
// lightweight equivalent using the same dropdown pattern as
// SkillPicker. When shadcn init runs, this component can be swapped
// to the shadcn primitive without changing the public API.
//
// Threat model:
//   - T-5-15 — the 全部重载 CTA triggers a fsnotify rescan; this is
//     the documented entry point (V12 symlink guard + Validate re-run
//     happen BE-side per 05-01 atomic.Pointer).
import { useEffect, useRef, useState } from 'react';
import { CheckCircle2, ChevronDown, FileWarning, RefreshCw, Wand2 } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import skillsData from '../../__fixtures__/skills.json';

interface SkillFixtureEntry {
  id: string;
  name: string;
  description: string;
}

const SKILLS: SkillFixtureEntry[] = skillsData as SkillFixtureEntry[];

export interface SkillManagerMenuProps {
  /** Override the path to POST for `全部重载`. */
  reloadPath?: string;
}

export default function SkillManagerMenu({ reloadPath = '/api/skills/reload' }: SkillManagerMenuProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const skillReloadStatus = useAppStore((s) => s.skillReloadStatus);
  const setCenterView = useAppStore((s) => s.setCenterView);
  const setWizardMode = useAppStore((s) => s.setWizardMode);

  useEffect(() => {
    if (!open) return;
    function onMouseDown(e: MouseEvent) {
      const target = e.target as Node | null;
      if (!target || !ref.current) return;
      if (!ref.current.contains(target)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [open]);

  function handleReloadAll() {
    // Fire-and-forget. The BE fsnotify rescan (V12 guarded) emits
    // skill-reloaded SSE events that ReloadToast surfaces. Failure
    // surfaces as a reload-failed toast.
    void fetch(reloadPath, { method: 'POST', credentials: 'same-origin' }).catch(() => undefined);
    setOpen(false);
  }

  function handleNewSkill() {
    // SC-3 — wizard entry. Sets wizardMode and routes the center to
    // the Skill 定义 tab. The full wizard UI ships in 05-04; this
    // gate marks the entry point + the click handler.
    setWizardMode({ active: true, step: 1, totalSteps: 5 });
    setCenterView({ kind: 'skill-md', skillId: SKILLS[0]?.id ?? 'compliance-review' });
    setOpen(false);
  }

  return (
    <div data-testid="skill-manager-menu" className="relative" ref={ref}>
      <button
        type="button"
        data-testid="skill-manager-trigger"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-xs font-semibold text-text hover:bg-surface-2"
        aria-label="Skill 管理"
      >
        <span>Skill 管理</span>
        <ChevronDown className="h-3.5 w-3.5" />
      </button>
      {open && (
        <div
          data-testid="skill-manager-dropdown"
          className="absolute right-0 z-20 mt-1 w-72 rounded-md border border-border bg-surface-2 shadow-lg"
        >
          <div className="px-3 pt-2 pb-1 text-xs font-semibold text-text-muted">已安装 Skill</div>
          <ul className="px-1 pb-2">
            {SKILLS.length === 0 && (
              <li
                data-testid="skill-manager-empty"
                className="px-2 py-1 text-xs text-text-muted"
              >
                尚未安装任何 Skill。点击「新建 Skill（向导）」创建。
              </li>
            )}
            {SKILLS.map((skill) => {
              const status = skillReloadStatus[skill.id];
              const failed = status?.status === 'failed';
              return (
                <li
                  key={skill.id}
                  data-testid="skill-manager-item"
                  data-skill-id={skill.id}
                  className={cn(
                    'flex items-center justify-between gap-2 rounded-md px-2 py-1 text-xs hover:bg-surface-3',
                  )}
                  title={
                    failed && status?.error
                      ? `加载失败：${status.error}`
                      : undefined
                  }
                >
                  <div className="min-w-0">
                    <div className="truncate text-text">{skill.name}</div>
                    <div className="flex items-center gap-1 text-xs text-text-muted">
                      {failed ? (
                        <>
                          <FileWarning className="h-3 w-3 text-status-non-compliant" />
                          <span>加载失败（旧版可用）</span>
                        </>
                      ) : (
                        <>
                          <CheckCircle2 className="h-3 w-3 text-status-compliant" />
                          <span>已加载</span>
                        </>
                      )}
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
          <div className="border-t border-border px-1 py-1 space-y-0.5">
            <button
              type="button"
              data-testid="skill-manager-reload-all"
              onClick={handleReloadAll}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs text-text hover:bg-surface-3"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              <span>全部重载</span>
            </button>
            <button
              type="button"
              data-testid="skill-manager-new-skill"
              onClick={handleNewSkill}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs text-text hover:bg-surface-3"
            >
              <Wand2 className="h-3.5 w-3.5" />
              <span>新建 Skill（向导）</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
