// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — vitest cases for useSkillAutoSwitch.
//
//   - TestUseSkillAutoSwitch_SetsSkillOnMount — pre-seed skills with
//     "compliance"; render with skillId="compliance"; assert
//     store.currentSkillId === "compliance".
//   - TestUseSkillAutoSwitch_NoOpWhenSkillNotFound — skillId="ghost";
//     assert currentSkillId stays unchanged.
//   - TestUseSkillAutoSwitch_NoOpWhenEmpty — skillId=""; assert
//     unchanged.

import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { useAppStore } from '../store/app-store';
import { useSkillAutoSwitch } from './use-skill-auto-switch';

afterEach(() => {
  // Reset the store between tests so test order is independent.
  useAppStore.setState({
    skills: [],
    currentSkillId: null,
  });
});

beforeEach(() => {
  useAppStore.setState({
    skills: [],
    currentSkillId: null,
  });
});

describe('useSkillAutoSwitch', () => {
  it('TestUseSkillAutoSwitch_SetsSkillOnMount — sets currentSkillId when the skill is in the loaded list', () => {
    useAppStore.setState({
      skills: [{ id: 'compliance', name: 'compliance', description: '', status: 'ok' }],
      currentSkillId: null,
    });
    renderHook(() => useSkillAutoSwitch('compliance'));
    expect(useAppStore.getState().currentSkillId).toBe('compliance');
  });

  it('TestUseSkillAutoSwitch_NoOpWhenSkillNotFound — currentSkillId stays unchanged', () => {
    useAppStore.setState({
      skills: [{ id: 'compliance', name: 'compliance', description: '', status: 'ok' }],
      currentSkillId: 'compliance',
    });
    renderHook(() => useSkillAutoSwitch('ghost'));
    expect(useAppStore.getState().currentSkillId).toBe('compliance');
  });

  it('TestUseSkillAutoSwitch_NoOpWhenEmpty — empty skillId leaves the store unchanged', () => {
    useAppStore.setState({
      skills: [{ id: 'compliance', name: 'compliance', description: '', status: 'ok' }],
      currentSkillId: 'compliance',
    });
    renderHook(() => useSkillAutoSwitch(''));
    expect(useAppStore.getState().currentSkillId).toBe('compliance');
  });

  // UAT 2026-09-06 B2 (P1) regression lock — a manual SkillPicker override
  // must PERSIST. The old implementation re-ran the auto-switch on every
  // currentSkillId change (dep array contained it), instantly snapping the
  // user's explicit selection back to the session-bound skill.
  it('TestUseSkillAutoSwitch_UserOverridePersists — manual switch is not snapped back', async () => {
    useAppStore.setState({
      skills: [
        { id: 'compliance', name: 'compliance', description: '', status: 'ok' },
        { id: 'simple-qa', name: 'simple-qa', description: '', status: 'ok' },
      ],
      currentSkillId: null,
    });
    renderHook(() => useSkillAutoSwitch('compliance'));
    expect(useAppStore.getState().currentSkillId).toBe('compliance');

    // The user explicitly picks a different Skill in the picker.
    useAppStore.getState().setCurrentSkill('simple-qa');

    // Polling refreshes the skills array identity; the hook must NOT
    // re-assert the session-bound skill over the user's override.
    await waitFor(() => {
      expect(useAppStore.getState().currentSkillId).toBe('simple-qa');
    });
    expect(useAppStore.getState().currentSkillId).toBe('simple-qa');
  });

  it('TestUseSkillAutoSwitch_ReappliesOnNewRestorePayload — a changed session binding applies once more', () => {
    useAppStore.setState({
      skills: [
        { id: 'compliance', name: 'compliance', description: '', status: 'ok' },
        { id: 'simple-qa', name: 'simple-qa', description: '', status: 'ok' },
      ],
      currentSkillId: null,
    });
    const { rerender } = renderHook(({ sid }) => useSkillAutoSwitch(sid), {
      initialProps: { sid: 'compliance' },
    });
    expect(useAppStore.getState().currentSkillId).toBe('compliance');

    // Switching sessions hands the hook a NEW restore payload — it applies.
    rerender({ sid: 'simple-qa' });
    expect(useAppStore.getState().currentSkillId).toBe('simple-qa');
  });
});