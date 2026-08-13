// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 + Plan 05-08 Task 2 — useSkillReload tests.
//
// Pins the SC-2 acceptance assertion that the hook updates the store
// when a skill-reloaded SSE payload arrives (or when the polling
// fallback detects a version bump).
//
// Plan 05-08 (WR-03) — the manifest test uses a callIdx counter that
// returns responses[idx] ?? responses[0] (not responses[0] blindly) so
// the second tick does not spuriously return [] and wipe the seeded
// skills. The setTimeout(60) is replaced with vi.waitFor against
// useAppStore.getState().skills for timing resilience.
//
// Plan 05-08 (CR-02) — new test pins that transient empty responses
// do NOT wipe live store.skills (the guard added in tick()).
//
// Plan 05-08 (IN-04) — new test pins that 3 consecutive non-OK
// responses flip skillServiceOffline=true.
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
  useAppStore.setState({
    skillReloadStatus: {},
    skills: [],
    skillServiceOffline: false,
  });
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

  it('polling detects a status flip (ok→failed) with no version bump (keep-old)', async () => {
    // Sequence: baseline v1 ok, then a broken-YAML reload → status flips
    // to failed with version UNCHANGED (keep-old contract, pinned by T4/T6).
    // The store entry must reflect status:'failed' / version:'old-kept' /
    // error carrying the validation message.
    const responses: Array<
      Array<{ id: string; name: string; version: number; status: 'ok' | 'failed'; error?: string }>
    > = [
      [{ id: 'compliance-review', name: 'compliance', version: 1, status: 'ok' }],
      [
        {
          id: 'compliance-review',
          name: 'compliance',
          version: 1,
          status: 'failed',
          error: 'temperature must be ≤ 2',
        },
      ],
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

    await new Promise((r) => setTimeout(r, 120));

    const entry = useAppStore.getState().skillReloadStatus['compliance-review'];
    expect(entry).toBeDefined();
    expect(entry?.status).toBe('failed');
    expect(entry?.version).toBe('old-kept');
    expect(entry?.error).toContain('temperature');
  });

  it('Plan 05-07 (WR-03 hardened) — populates store.skills from manifest.description (NOT item.description)', async () => {
    // BLOCKING CONSTRAINT (.continue-here.md): GET /api/skills returns items
    // shaped {id,name,version,status,error?,manifest} where the 7 schema
    // fields live NESTED under manifest. The poller MUST read
    // item.manifest.description (reading item.description directly is the
    // bug this plan fixes — yields undefined).
    //
    // Plan 05-08 WR-03 — use a callIdx counter that returns
    // responses[idx] ?? responses[0] so the second tick does not
    // spuriously return [] (which would trigger the CR-02 guard and
    // leave the list intact, but used to leave the timing-dependent
    // mask exposed). vi.waitFor asserts against the actual store state
    // instead of wall-clock time.
    const responses: Array<
      Array<{
        id: string;
        name: string;
        version: number;
        status: 'ok';
        manifest: { description: string };
      }>
    > = [
      [
        {
          id: 's1',
          name: 'S1',
          version: 1,
          status: 'ok',
          manifest: { description: 'desc-from-manifest' },
        },
      ],
    ];
    let callIdx = 0;
    const fetchMock = vi.fn().mockImplementation(async () => {
      const idx = callIdx++;
      return {
        ok: true,
        json: async () => responses[idx] ?? responses[0],
      } as unknown as Response;
    });
    vi.stubGlobal('fetch', fetchMock);

    function Probe() {
      useSkillReload({ disablePolling: false, pollIntervalMs: 30, pollPath: '/api/test/skills' });
      return null;
    }
    render(<Probe />);

    // Wait for the assertion to hold (not wall-clock).
    await vi.waitFor(
      () => {
        const skills = useAppStore.getState().skills;
        expect(skills).toHaveLength(1);
        expect(skills[0].id).toBe('s1');
        expect(skills[0].description).toBe('desc-from-manifest');
        expect(skills[0].status).toBe('ok');
      },
      { timeout: 1000 },
    );
  });

  it('Plan 05-08 CR-02 — transient empty /api/skills response does NOT wipe live store.skills', async () => {
    // Seed a live skill so the transient-empty guard has something to
    // protect. Mock fetch to return [] on every call — even after the
    // guard kicks in, the seeded skill must remain.
    useAppStore.setState({
      skills: [{ id: 's1', name: 'S1', description: 'd', status: 'ok' }],
    });
    const fetchMock = vi.fn().mockImplementation(async () => ({
      ok: true,
      json: async () => [],
    } as unknown as Response));
    vi.stubGlobal('fetch', fetchMock);

    function Probe() {
      useSkillReload({ disablePolling: false, pollIntervalMs: 30, pollPath: '/api/test/skills' });
      return null;
    }
    render(<Probe />);

    // Wait long enough for several ticks to fire; the seeded skill
    // must persist across all of them.
    await vi.waitFor(
      () => {
        const skills = useAppStore.getState().skills;
        expect(skills).toHaveLength(1);
        expect(skills[0].id).toBe('s1');
      },
      { timeout: 500 },
    );
    // Give the polling loop a few extra cycles to confirm the guard
    // is stable (not just lucky timing).
    await new Promise((r) => setTimeout(r, 120));
    const skills = useAppStore.getState().skills;
    expect(skills).toHaveLength(1);
    expect(skills[0].id).toBe('s1');
  });

  it('Plan 05-08 IN-04 — sets skillServiceOffline=true after 3 consecutive non-OK responses', async () => {
    // Mock fetch to return !ok on every call. The counter should
    // reach 3 (the threshold) and dispatch setSkillServiceOffline(true).
    const fetchMock = vi.fn().mockImplementation(async () => ({
      ok: false,
      json: async () => [],
    } as unknown as Response));
    vi.stubGlobal('fetch', fetchMock);

    function Probe() {
      useSkillReload({ disablePolling: false, pollIntervalMs: 30, pollPath: '/api/test/skills' });
      return null;
    }
    render(<Probe />);

    // Wait for the counter to reach the threshold.
    await vi.waitFor(
      () => {
        expect(useAppStore.getState().skillServiceOffline).toBe(true);
      },
      { timeout: 1000 },
    );
    expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(3);
  });
});
