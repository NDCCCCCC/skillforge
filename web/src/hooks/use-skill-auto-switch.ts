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

import { useEffect, useRef } from 'react';
import { useAppStore } from '../store/app-store';

/**
 * Auto-switch SkillPicker to the session-bound skill on history
 * restore. Pass the `skill_id` field from the GET /api/session/:id/history
 * response. Empty / unknown skillId is a no-op.
 *
 * UAT 2026-09-06 B2 (P1) — the restore payload is applied AT MOST ONCE per
 * distinct skillId. The previous implementation kept `currentSkillId` in the
 * dependency array with an equality idempotency check, so every MANUAL
 * SkillPicker override re-fired the effect, saw `currentSkillId !== skillId`,
 * and snapped the user's explicit selection back to the session-bound skill —
 * making skill switching impossible on any session with restored history.
 * A `useRef` ledger replaces the equality check: the effect only fires when
 * the restore payload itself changes (session switch) or when the skill list
 * first contains the bound skill (delayed load).
 */
export function useSkillAutoSwitch(skillId: string | null | undefined): void {
  const skills = useAppStore((s) => s.skills);
  const setCurrentSkill = useAppStore((s) => s.setCurrentSkill);
  // Ledger of the last restore payload this hook applied (or is waiting to
  // apply). Deliberately NOT applied-skill state — user overrides never
  // touch it, so they can never re-trigger the auto-switch.
  const appliedRef = useRef<string | null>(null);

  useEffect(() => {
    if (!skillId) return;
    // No-op when this exact restore payload was already applied
    // (idempotency — the hook re-fires on every skills-list refresh).
    if (appliedRef.current === skillId) return;
    // No-op when the skill is not in the loaded list (skills.json
    // hasn't loaded yet OR the session references a Skill the user
    // does not have locally). The hook re-fires when skills
    // refreshes, so a delayed Skills load will still trigger the
    // auto-switch.
    const known = skills.some((s) => s.id === skillId);
    if (!known) return;
    appliedRef.current = skillId;
    setCurrentSkill(skillId);
  }, [skillId, skills, setCurrentSkill]);
}