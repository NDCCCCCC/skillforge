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
import { renderHook } from '@testing-library/react';
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
});