// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — useSkillReload tests.
//
// Pins the SC-2 acceptance assertion that the hook updates the store
// when a skill-reloaded SSE payload arrives (or when the polling
// fallback detects a version bump).
import { cleanup, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { useSkillReload } from '../hooks/useSkillReload';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
beforeEach(() => {
  useAppStore.setState({ skillReloadStatus: {} });
});

describe('useSkillReload', () => {
  it('mounts without throwing and seeds no skill-reloaded entries by default', () => {
    function Probe() {
      useSkillReload({ disablePolling: true });
      return null;
    }
    render(<Probe />);
    expect(useAppStore.getState().skillReloadStatus).toEqual({});
  });

  it('polling fallback detects a version bump and updates the store', async () => {
    // Sequential responses: first call returns v1 (baseline), second
    // returns v2 (the bump). Use a queue keyed by call index.
    const responses: Array<Array<{ id: string; name: string; version: number }>> = [
      [{ id: 'compliance-review', name: 'compliance', version: 1 }],
      [{ id: 'compliance-review', name: 'compliance', version: 2 }],
    ];
    let callIdx = 0;
    const fetchMock = vi.fn().mockImplementation(async () => {
      const idx = callIdx++;
      return {
        ok: true,
        json: async () => responses[idx] ?? [],
      } as unknown as Response;
    });
    vi.stubGlobal('fetch', fetchMock);

    function Probe() {
      useSkillReload({ disablePolling: false, pollIntervalMs: 30, pollPath: '/api/test/skills' });
      return null;
    }
    render(<Probe />);

    // Wait for both ticks to complete (initial tick + one interval tick).
    await new Promise((r) => setTimeout(r, 120));

    const status = useAppStore.getState().skillReloadStatus;
    expect(status['compliance-review']).toBeDefined();
    expect(status['compliance-review']?.status).toBe('ok');
    expect(status['compliance-review']?.version).toBe('new');
    // Polling ran at least twice (initial tick + interval tick); we
    // don't assert the exact count because the interval is timer-based
    // and may fire extra times within the wait window.
    expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
