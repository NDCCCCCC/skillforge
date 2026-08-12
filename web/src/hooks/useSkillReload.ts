// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// useSkillReload — Plan 05-03 / SC-2 (SYSK-04) hot-reload consumer.
//
// Subscribes to the SSE channel `GET /api/skills/events` and updates
// `store.skillReloadStatus` with each `skill-reloaded` payload:
//
//   { type: 'skill-reloaded', skillId, skillName, status, error?, version }
//
// Strategy (per UI-SPEC §SC-2):
//   - Primary path: SSE EventSource on `/api/skills/events` with exp
//     backoff (3s base, 30s cap) via sse-client.constructSSE.
//   - Fallback: when EventSource is unavailable (jsdom / dev-only
//     mode), poll `GET /api/skills` every 2s and diff the per-skill
//     `version` field. If the version increments AND status changes,
//     emit a synthetic reload event.
//
// Threat model:
//   - T-5-11 — reconnect storm mitigated by sse-client exp backoff.
//   - T-5-12 — spoofed toast cannot affect routing (toast is informational,
//     actual Skill reload is BE-side per D5-08 + 05-01 atomic.Pointer).
//   - T-5-14 — validation errors from the BE are text-rendered only;
//     filesystem paths are stripped to their basename by the ReloadToast
//     consumer (defense-in-depth).
//
// Mount at the app root so the subscription outlives route changes.
import { useEffect } from 'react';
import { constructSSE } from '../lib/sse-client';
import { useAppStore } from '../store/app-store';
import type { SkillReloadEntry } from '../store/app-store';

interface SkillReloadedPayload {
  type: 'skill-reloaded';
  skillId: string;
  skillName: string;
  status: 'ok' | 'failed';
  version?: 'new' | 'old-kept';
  error?: string;
}

interface SkillListItem {
  id: string;
  name?: string;
  version?: number;
}

/** Coerce a payload into a SkillReloadEntry. Defensive — accepts the
 * SSE payload shape with optional fields and falls back to safe defaults. */
function toReloadEntry(payload: Partial<SkillReloadedPayload> & { skillId: string }): SkillReloadEntry {
  const status: SkillReloadEntry['status'] = payload.status === 'failed' ? 'failed' : 'ok';
  const version: SkillReloadEntry['version'] = payload.version === 'old-kept' ? 'old-kept' : 'new';
  return {
    status,
    version,
    error: typeof payload.error === 'string' ? payload.error : undefined,
    ts: Date.now(),
    skillName: typeof payload.skillName === 'string' ? payload.skillName : payload.skillId,
  };
}

export interface UseSkillReloadOptions {
  /** Override the SSE path (default `/api/skills/events`). */
  ssePath?: string;
  /** Override the polling fallback URL (default `/api/skills`). */
  pollPath?: string;
  /** Polling cadence when SSE is unavailable (default 2000 ms). */
  pollIntervalMs?: number;
  /** Disable the polling fallback entirely (default false). */
  disablePolling?: boolean;
}

/**
 * Mount once at the app root. Returns nothing — the hook's effect
 * drives the subscription and updates the Zustand store. ReloadToast
 * observes the store and surfaces toasts.
 */
export function useSkillReload(opts: UseSkillReloadOptions = {}): void {
  const ssePath = opts.ssePath ?? '/api/skills/events';
  const pollPath = opts.pollPath ?? '/api/skills';
  const pollIntervalMs = opts.pollIntervalMs ?? 2000;

  useEffect(() => {
    let cancelled = false;

    function handleEvent(payload: unknown) {
      if (!payload || typeof payload !== 'object') return;
      const p = payload as Partial<SkillReloadedPayload>;
      if (p.type !== 'skill-reloaded' || typeof p.skillId !== 'string') return;
      const entry = toReloadEntry({ ...p, skillId: p.skillId });
      if (!cancelled) {
        useAppStore.getState().setSkillReloadStatus(p.skillId, entry);
      }
    }

    // Primary path: SSE with auto-reconnect.
    const conn = constructSSE({
      path: ssePath,
      onMessage: handleEvent,
      // Suppress uncaught errors during dev — the sse-client swallows
      // them and schedules a reconnect. Production telemetry should
      // wire opts.onError to a logger.
      onError: () => undefined,
    });
    conn.connect();

    // Fallback: polling — runs alongside SSE. If both fire, the store
    // updates are idempotent (same skillId, latest ts wins).
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    const knownVersions = new Map<string, number>();

    async function tick() {
      if (cancelled) return;
      try {
        const res = await fetch(pollPath, { credentials: 'same-origin' });
        if (!res.ok) return;
        const list = (await res.json()) as SkillListItem[];
        for (const item of list) {
          const prev = knownVersions.get(item.id);
          if (prev !== undefined && typeof item.version === 'number' && item.version > prev) {
            handleEvent({
              type: 'skill-reloaded',
              skillId: item.id,
              skillName: item.name ?? item.id,
              status: 'ok',
              version: 'new',
            });
          }
          if (typeof item.version === 'number') {
            knownVersions.set(item.id, item.version);
          }
        }
      } catch {
        // Network failure — silently retry on next tick.
      }
    }

    if (!opts.disablePolling) {
      // Kick off one tick immediately so the first poll captures the
      // baseline versions before any change.
      void tick();
      pollTimer = setInterval(() => void tick(), pollIntervalMs);
    }

    return () => {
      cancelled = true;
      conn.close();
      if (pollTimer !== null) clearInterval(pollTimer);
    };
    // Re-subscribe if paths change (rare; included for completeness).
  }, [ssePath, pollPath, pollIntervalMs, opts.disablePolling]);
}
