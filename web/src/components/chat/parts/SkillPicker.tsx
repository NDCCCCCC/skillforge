// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillPicker — FEUI-04 / SYSK-03 skill dropdown. Open Design redesign
// B3: the list is the LIVE skill surface (useAppStore.skills — seeded
// by useSkillReload from GET /api/skills), not the skills.json
// fixture. Selecting a skill dispatches `setCurrentSkill(id)` and also
// `setCenterView({ kind: 'skill-md', skillId })` so the detail panel
// swaps to the SKILL.md preview (D-02).
//
// Plan 05-08 (IN-04) — the skill-service-offline banner moved from the
// deleted LeftSidebar into the dropdown head (loud-failure signal).
//
// Plan 05-03 (D5-03) — ambiguity picker mode. When the `scores` prop
// is provided, the component switches into "ambiguity picker" mode:
//   - The candidate list is `scores` (NOT the live skill surface)
//   - Each row shows the cosine score badge
//   - Heading copy is `检测到歧义，请选择目标 Skill`
//   - Per-candidate CTA `使用此 Skill` instead of the default select
//   - A `使用默认 Skill` dismiss button appears at the bottom
import { useEffect, useRef, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { useAppStore } from '../../../store/app-store';
import type { SkillScore } from '../../../types/wire-types';

export interface SkillPickerProps {
  /**
   * Plan 05-03 — when present, the picker renders in ambiguity mode.
   * Each SkillScore row exposes a `使用此 Skill` CTA. When omitted,
   * the picker renders the default skill-switching dropdown.
   */
  scores?: SkillScore[];
  /** Plan 05-03 — invoked when the user picks a candidate in ambiguity mode. */
  onPickAmbiguous?: (skillId: string) => void;
  /** Plan 05-03 — invoked when the user clicks `使用默认 Skill`. */
  onDismissAmbiguous?: () => void;
}

export default function SkillPicker({ scores, onPickAmbiguous, onDismissAmbiguous }: SkillPickerProps) {
  const ambiguityMode = Boolean(scores && scores.length > 0);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const skills = useAppStore((s) => s.skills);
  const skillServiceOffline = useAppStore((s) => s.skillServiceOffline);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = skills.find((s) => s.id === currentSkillId);

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

  function handleSelect(skillId: string) {
    useAppStore.getState().setCurrentSkill(skillId);
    useAppStore.getState().setCenterView({ kind: 'skill-md', skillId });
    setOpen(false);
  }

  function handleAmbiguousPick(skillId: string) {
    onPickAmbiguous?.(skillId);
    setOpen(false);
  }

  function handleDismiss() {
    onDismissAmbiguous?.();
    setOpen(false);
  }

  // Render heading + trigger copy according to the operating mode.
  const triggerLabel = ambiguityMode ? '检测到歧义，请选择目标 Skill' : current?.name ?? '选择 Skill';

  return (
    <div data-testid="skill-picker" data-mode={ambiguityMode ? 'ambiguity' : 'default'} className="relative" ref={ref}>
      <button
        type="button"
        data-testid="skill-picker-trigger"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex w-full items-center justify-between gap-2 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text"
      >
        <span className="truncate">{triggerLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0" />
      </button>

      {open && (
        <div
          data-testid="skill-picker-menu"
          className="absolute z-10 mt-1 w-full min-w-56 rounded-md border border-border bg-surface-2 shadow-lg"
        >
          {skillServiceOffline && !ambiguityMode && (
            <div
              data-testid="skill-picker-offline-banner"
              className="border-b border-border px-2 py-1 text-xs text-status-needs-info"
            >
              Skill 服务暂不可达，列表可能不是最新
            </div>
          )}
          {ambiguityMode ? (
            <>
              {(scores ?? []).map((s) => (
                <div
                  key={s.skillId}
                  data-testid="skill-picker-item"
                  data-skill-id={s.skillId}
                  data-score={s.score.toFixed(2)}
                  className={cn(
                    'block w-full px-2 py-1 text-left text-xs hover:bg-surface-3',
                    s.selected && 'bg-primary-dim',
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-text">{s.skillName}</div>
                      <div className="font-mono text-xs text-text-muted">
                        cos {s.score.toFixed(2)}
                      </div>
                    </div>
                    <button
                      type="button"
                      data-testid="skill-picker-cta"
                      onClick={() => handleAmbiguousPick(s.skillId)}
                      className="rounded-md border border-border bg-surface px-2 py-0.5 text-xs font-semibold text-primary hover:bg-surface-3"
                    >
                      使用此 Skill
                    </button>
                  </div>
                </div>
              ))}
              <button
                type="button"
                data-testid="skill-picker-dismiss"
                onClick={handleDismiss}
                className="block w-full border-t border-border px-2 py-1 text-left text-xs text-text-muted hover:bg-surface-3 hover:text-text"
              >
                使用默认 Skill
              </button>
            </>
          ) : skills.length === 0 ? (
            <div className="px-2 py-1 text-xs text-text-muted">
              暂无已加载 Skill（可在 Skill 菜单中创建）
            </div>
          ) : (
            skills.map((skill) => {
              const active = skill.id === currentSkillId;
              return (
                <button
                  key={skill.id}
                  type="button"
                  data-testid={active ? 'skill-picker-item-active' : 'skill-picker-item'}
                  data-skill-id={skill.id}
                  onClick={() => handleSelect(skill.id)}
                  className={cn(
                    'block w-full px-2 py-1 text-left text-xs hover:bg-surface-3',
                    active && 'bg-primary-dim',
                  )}
                >
                  <div className="truncate">{skill.name}</div>
                  {skill.description && (
                    <div className="truncate text-xs text-text-muted">{skill.description}</div>
                  )}
                </button>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
