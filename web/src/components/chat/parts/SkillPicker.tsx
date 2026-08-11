// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillPicker — FEUI-04 / SYSK-03 skill dropdown. Reads the list of
// skills from skills.json (Vite JSON import), shows the current skill
// name as a trigger button, and opens a menu listing every skill on
// click. Selecting a skill dispatches `setCurrentSkill(id)` and also
// `setCenterView({ kind: 'skill-md', skillId })` so the center panel
// swaps to the SKILL.md preview (D-02).
import { useEffect, useRef, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { useAppStore } from '../../../store/app-store';
import skillsData from '../../../__fixtures__/skills.json';

interface SkillFixtureEntry {
  id: string;
  name: string;
  description: string;
}

const SKILLS: SkillFixtureEntry[] = skillsData as SkillFixtureEntry[];

export default function SkillPicker() {
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = SKILLS.find((s) => s.id === currentSkillId);

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

  function handleSelect(skill: SkillFixtureEntry) {
    useAppStore.getState().setCurrentSkill(skill.id);
    useAppStore.getState().setCenterView({ kind: 'skill-md', skillId: skill.id });
    setOpen(false);
  }

  return (
    <div data-testid="skill-picker" className="relative" ref={ref}>
      <button
        type="button"
        data-testid="skill-picker-trigger"
        onClick={() => setOpen((v) => !v)}
        className="w-full inline-flex items-center justify-between gap-2 rounded-md bg-surface-2 text-text border border-border px-2 py-1 text-xs font-semibold"
      >
        <span>{current?.name ?? '选择 Skill'}</span>
        <ChevronDown className="h-3.5 w-3.5" />
      </button>

      {open && (
        <div
          data-testid="skill-picker-menu"
          className="absolute z-10 mt-1 w-full rounded-md border border-border bg-surface-2 shadow-lg"
        >
          {SKILLS.map((skill) => {
            const active = skill.id === currentSkillId;
            return (
              <button
                key={skill.id}
                type="button"
                data-testid={active ? 'skill-picker-item-active' : 'skill-picker-item'}
                data-skill-id={skill.id}
                onClick={() => handleSelect(skill)}
                className={cn(
                  'block w-full text-left px-2 py-1 text-xs hover:bg-surface-3',
                  active && 'bg-primary-dim',
                )}
              >
                <div>{skill.name}</div>
                <div className="text-text-muted text-xs">{skill.description}</div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
