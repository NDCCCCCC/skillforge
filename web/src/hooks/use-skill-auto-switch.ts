// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — useSkillAutoSwitch hook.
//
// On cross-refresh restore, the BE returns the session-bound skill_id
// (Plan 03's GET /api/session/:id/history response). The SkillPicker
// in the chat surface MUST auto-switch to that skill so the next
// query routes against the same Skill the session was built under
// (D10-D2: 恢复历史时 FE 自动切换到该 session 绑定的 Skill).
//
// Contract:
//   - useSkillAutoSwitch(skillId) reads the current skill list from
//     the Zustand store; when skillId is non-empty AND is present in
//     the loaded skills list, it calls store.setCurrentSkill(skillId).
//   - Idempotent: no-op when currentSkillId already matches skillId.
//   - Skips silently when skillId is empty / not in the loaded list
//     (the user may not have loaded the Skills fixture yet; the hook
//     re-runs when skills refreshes so it picks up the match later).
//   - The dependency array is [skillId, skills] so the hook re-fires
//     when either input changes — the user might have just loaded a
//     new Skill that matches the session's history.

import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';

/**
 * Auto-switch SkillPicker to the session-bound skill on history
 * restore. Pass the `skill_id` field from the GET /api/session/:id/history
 * response. Empty / unknown skillId is a no-op.
 */
export function useSkillAutoSwitch(skillId: string | null | undefined): void {
  const skills = useAppStore((s) => s.skills);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const setCurrentSkill = useAppStore((s) => s.setCurrentSkill);

  useEffect(() => {
    if (!skillId) return;
    // No-op when already set (idempotency — the hook may fire on
    // every skills-list refresh).
    if (currentSkillId === skillId) return;
    // No-op when the skill is not in the loaded list (skills.json
    // hasn't loaded yet OR the session references a Skill the user
    // does not have locally). The hook re-fires when skills
    // refreshes, so a delayed Skills load will still trigger the
    // auto-switch.
    const known = skills.some((s) => s.id === skillId);
    if (!known) return;
    setCurrentSkill(skillId);
  }, [skillId, skills, currentSkillId, setCurrentSkill]);
}