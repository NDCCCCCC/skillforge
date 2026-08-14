// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillManagerMenu — Plan 05-03 + Plan 05-07 topbar dropdown.
//
// Plan 05-07 (D5-IA-04 + D5-IA-05) reduces the menu to the wizard
// entry only:
//   - 全部重载 button REMOVED — D5-IA-05 deletes the dead POST
//     /api/skills/reload route (a 404). Hot reload is now polling-only
//     per §2 decision B, surfacing via useSkillReload + ReloadToast.
//   - Per-skill list (skill-manager-item rows + status badges) MOVED
//     to the LeftSidebar 技能 block per D5-IA-04 division of labor.
//     That block reads store.skills (live GET /api/skills via
//     useSkillReload).
//   - The 新建 Skill（向导） entry (skill-manager-new-skill) remains
//     and is the sole entry point. It opens the SC-3 wizard.
//
// Threat model:
//   - No fetch is issued from this component — D5-IA-05. Reload
//     signalling is owned by useSkillReload (polling) + ReloadToast
//     (toast UI). No dead-end POST /api/skills/reload roundtrip.
import { useEffect, useRef, useState } from 'react';
import { ChevronDown, Wand2 } from 'lucide-react';
import { useAppStore } from '../../store/app-store';

export default function SkillManagerMenu() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
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

  function handleNewSkill() {
    // SC-3 — wizard entry. Sets wizardMode and routes the center to
    // the Skill 定义 tab. The full wizard UI ships in 05-04; this
    // gate marks the entry point + the click handler.
    setWizardMode({ active: true, step: 1, totalSteps: 5 });
    setCenterView({ kind: 'skill-md', skillId: 'simple-qa' });
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
          className="absolute right-0 z-20 mt-1 w-56 rounded-md border border-border bg-surface-2 shadow-lg"
        >
          <div className="border-t border-border px-1 py-1">
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