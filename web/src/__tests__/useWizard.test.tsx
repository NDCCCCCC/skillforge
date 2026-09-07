// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — useWizard state machine tests.
//
// Pins the SC-3 hook contract:
//   1. Hook starts idle until store.wizardMode.active flips true.
//   2. submitAnswer advances step + records the answer; stops at totalSteps.
//   3. requestPreview POSTs to /api/skills/wizard with the collected
//      answers; on success, transitions to previewing phase.
//   4. requestPreview error surfaces in error state without changing phase.
//   5. commit POSTs to /api/skills/wizard/commit; on ok=true fires
//      'wizard-success' CustomEvent + exits wizard mode.

import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { useWizard } from '../hooks/useWizard';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  useAppStore.setState({
    wizardMode: { active: false, step: 0, totalSteps: 0 },
  });
});

beforeEach(() => {
  useAppStore.setState({
    wizardMode: { active: false, step: 0, totalSteps: 0 },
  });
});

const VALID_PREVIEW = {
  description: '审批流合规审计 Skill',
  agent: 'ChatModelAgent',
  reads: ['authority/**'],
  writes: [],
  steps: '1. step',
  guardrails: {
    prohibitions: ['a', 'b', 'c'],
    required_refusal_phrases: ['no_authorized_source'],
  },
  temperature: 0,
};

describe('useWizard', () => {
  it('starts in idle phase until wizardMode.active becomes true', () => {
    const { result } = renderHook(() => useWizard());
    expect(result.current.phase).toBe('idle');
    expect(result.current.currentPrompt).toBe('');
  });

  it('transitions to answering on wizardMode.active=true and exposes the first prompt', () => {
    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    expect(result.current.phase).toBe('answering');
    expect(result.current.step).toBe(0);
    expect(result.current.currentPrompt).toContain('业务问题');
  });

  it('submitAnswer records the answer for the current step + advances', () => {
    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();

    act(() => {
      result.current.submitAnswer('Compliance audit for policy reviews.');
    });
    rerender();
    expect(result.current.answers.purpose).toBe('Compliance audit for policy reviews.');
    expect(result.current.step).toBe(1);
    // 26-08: the reads prompt speaks the registry vocabulary (slugs / roles).
    expect(result.current.currentPrompt).toContain('知识库');
    expect(result.current.currentPrompt).toContain('authority');
  });

  it('submitAnswer parses reads as a comma-separated list', () => {
    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    act(() => {
      result.current.submitAnswer('Compliance audit for policy reviews.');
    });
    rerender();
    act(() => {
      result.current.submitAnswer('authority, history');
    });
    rerender();
    expect(result.current.answers.reads).toEqual(['authority', 'history']);
  });

  it('submitAnswer stops at totalSteps (sentinel = step === totalSteps)', () => {
    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();

    // The optional temperature step accepts empty input as "use the
    // type default" — but to keep the test deterministic we submit a
    // non-empty token for all 5 steps.
    const answers = [
      'Compliance audit for policy reviews.',
      'authority',
      'compliance',
      'Never modify authority',
      '0',
    ];
    for (const a of answers) {
      act(() => {
        result.current.submitAnswer(a);
      });
      rerender();
    }
    expect(result.current.step).toBe(5);
  });

  it('requestPreview POSTs the answers and transitions to previewing on success', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({
        preview: VALID_PREVIEW,
        name: 'compliance-audit',
      }),
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/skills/wizard',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(result.current.phase).toBe('previewing');
    expect(result.current.hasPreview).toBe(true);
    expect(result.current.preview?.name).toBe('compliance-audit');
  });

  it('requestPreview surfaces backend error without flipping phase', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({ error: 'purpose must contain an English keyword', name: '' }),
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();

    expect(result.current.phase).toBe('answering');
    expect(result.current.error).toContain('English keyword');
  });

  it('commit POSTs to /api/skills/wizard/commit, fires wizard-success, and exits wizard mode', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        json: async () => ({ preview: VALID_PREVIEW, name: 'compliance-audit' }),
      } as unknown as Response)
      .mockResolvedValueOnce({
        json: async () => ({
          ok: true,
          registered: true,
          skillsTotal: 2,
          path: '/skills/compliance-audit/SKILL.md',
        }),
      } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const onSuccess = vi.fn();
    window.addEventListener('wizard-success', onSuccess);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();
    await act(async () => {
      await result.current.commit();
    });
    rerender();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/skills/wizard/commit',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(useAppStore.getState().wizardMode.active).toBe(false);
    expect(result.current.phase).toBe('idle');
    expect(onSuccess).toHaveBeenCalled();
    const detail = (onSuccess.mock.calls[0][0] as CustomEvent).detail;
    expect(detail).toMatchObject({ skillName: 'compliance-audit' });

    window.removeEventListener('wizard-success', onSuccess);
  });

  it('commit DOES NOT fire wizard-success when ok=true but registered is false (UAT Test 9)', async () => {
    // Regression: when the BE wrote the file (ok=true) but could not
    // confirm registration (registered=false — e.g. Loader nil, rescan
    // failed, or the new name did not appear), the FE MUST NOT fire the
    // 'wizard-success' event (whose toast text claims "已注册并热加载").
    // The hook surfaces an accurate error instead and stays in the
    // previewing phase so the user can retry.
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        json: async () => ({ preview: VALID_PREVIEW, name: 'compliance-audit' }),
      } as unknown as Response)
      .mockResolvedValueOnce({
        json: async () => ({
          ok: true,
          registered: false,
          path: '/skills/compliance-audit/SKILL.md',
          error: 'skill written but no Loader is wired; registration not confirmed',
        }),
      } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const onSuccess = vi.fn();
    window.addEventListener('wizard-success', onSuccess);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();
    await act(async () => {
      await result.current.commit();
    });
    rerender();

    expect(
      onSuccess,
      'the false "已注册并热加载" toast MUST NOT fire when registered is false',
    ).not.toHaveBeenCalled();
    expect(result.current.phase).toBe('previewing');
    expect(result.current.error).toContain('registration not confirmed');
    expect(useAppStore.getState().wizardMode.active).toBe(true);

    window.removeEventListener('wizard-success', onSuccess);
  });

  it('commit surfaces backend error and stays in previewing phase', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        json: async () => ({ preview: VALID_PREVIEW, name: 'compliance-audit' }),
      } as unknown as Response)
      .mockResolvedValueOnce({
        json: async () => ({ ok: false, error: 'validation failed' }),
      } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();
    await act(async () => {
      await result.current.commit();
    });
    rerender();

    expect(result.current.phase).toBe('previewing');
    expect(result.current.error).toContain('validation');
    expect(useAppStore.getState().wizardMode.active).toBe(true);
  });

  it('cancel clears wizardMode.active regardless of phase', () => {
    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    act(() => {
      result.current.cancel();
    });
    expect(useAppStore.getState().wizardMode.active).toBe(false);
  });

  it('regenerate bounces back to answering phase + resets step', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({ preview: VALID_PREVIEW, name: 'compliance-audit' }),
    } as unknown as Response);
    vi.stubGlobal('fetch', fetchMock);

    const { result, rerender } = renderHook(() => useWizard());
    act(() => {
      useAppStore.getState().setWizardMode({ active: true, step: 1, totalSteps: 5 });
    });
    rerender();
    await act(async () => {
      await result.current.requestPreview();
    });
    rerender();
    expect(result.current.phase).toBe('previewing');

    act(() => {
      result.current.regenerate();
    });
    rerender();
    expect(result.current.phase).toBe('answering');
    expect(result.current.step).toBe(0);
    expect(result.current.hasPreview).toBe(false);
  });
});