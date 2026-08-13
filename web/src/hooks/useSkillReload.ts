// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// useSkillReload — Plan 05-03 / SC-2 (SYSK-04) hot-reload consumer
//                          + Plan 05-07 / D5-IA-01/04 live-skill surface.
//
// Polls `GET /api/skills` and updates `store.skillReloadStatus` whenever a
// skill's reload signal changes. The endpoint returns one item per skill:
//
//   { id, name, version, status, error?, manifest }
//
// Strategy (per UI-SPEC §SC-2, decision B — polling-only):
//   - Poll `GET /api/skills` every 2s (default) and diff each item's
//     `version` + `status` against the previous poll.
//   - Emit a synthetic reload event on EITHER:
//       (a) version increased — a successful reload bumped the per-process
//           monotonic counter (JS-safe, << 2^53), so every bump is visible
//           to the polling client; OR
//       (b) status flipped — ok→failed surfaces a broken-YAML reload with
//           the prior version KEPT; failed→ok surfaces a recovery.
//   - On a failure status the emitted entry carries version:'old-kept'
//     (the keep-old contract: a failed reload does NOT bump version), so
//     ReloadToast can say "校验失败，已保留上一版本" alongside the error text.
//
// Plan 05-07 extension: every tick ALSO normalizes the fetched list
// into SkillSummary[] and dispatches `setSkills(summaries)`. The list
// itself is NOT a reload signal — only version/status diffs emit toasts.
// The 7 schema fields live NESTED under item.manifest.* per the
// .continue-here.md BLOCKING CONSTRAINT; reading item.description
// directly yields undefined. We read item.manifest.description here.
//
// The first tick establishes a baseline per skill WITHOUT emitting —
// otherwise mounting the hook would spuriously fire a toast for every
// already-loaded skill.
//
// Threat model:
//   - T-5-11 — no SSE EventSource, so there is no reconnect storm. Polling
//     is a single setInterval; a failed fetch is retried silently on the
//     next tick (no exp backoff needed at this cadence).
//   - T-5-12 — a spoofed toast cannot affect routing (the toast is
//     informational; the actual Skill reload is BE-side per D5-08 +
//     05-01 atomic.Pointer).
//   - T-5-14 — validation errors from the BE are text-rendered only;
//     filesystem paths are stripped to their basename by the ReloadToast
//     consumer (defense-in-depth).
//
// Mount at the app root so polling outlives route changes.
import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';
import type { SkillReloadEntry, SkillSummary } from '../store/app-store';

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
  status?: 'ok' | 'failed';
  error?: string;
  /**
   * Plan 05-07 — manifest carries the 7 schema fields nested under it
   * per .continue-here.md BLOCKING CONSTRAINT. We widen SkillListItem
   * inline to access manifest.description for the SkillSummary surface.
   */
  manifest?: { description?: string };
}

/** Coerce a payload into a SkillReloadEntry. Defensive — accepts the
 * synthetic payload shape with optional fields and falls back to safe defaults. */
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
  /** Override the polling URL (default `/api/skills`). */
  pollPath?: string;
  /** Polling cadence (default 2000 ms). */
  pollIntervalMs?: number;
  /** Disable polling entirely (default false). */
  disablePolling?: boolean;
}

/**
 * Mount once at the app root. Returns nothing — the hook's effect
 * drives the poll loop and updates the Zustand store. ReloadToast
 * observes the store and surfaces toasts.
 */
export function useSkillReload(opts: UseSkillReloadOptions = {}): void {
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

    // Polling-only reload signal. Tracks the last-seen version + status
    // per skill so the first tick seeds a baseline WITHOUT emitting.
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    const known = new Map<string, { version?: number; status: 'ok' | 'failed' }>();

    async function tick() {
      if (cancelled) return;
      try {
        const res = await fetch(pollPath, { credentials: 'same-origin' });
        if (!res.ok) return;
        const list = (await res.json()) as SkillListItem[];
        // Plan 05-07 — normalize the list into SkillSummary[] and seed
        // store.skills. The list itself is NOT a reload signal — only
        // version/status diffs inside the loop emit toasts below.
        // item.manifest.description is the load-bearing field per
        // .continue-here.md BLOCKING CONSTRAINT; reading item.description
        // directly yields undefined (the bug this plan fixes).
        const summaries: SkillSummary[] = list
          .filter((item): item is SkillListItem & { id: string } => typeof item.id === 'string')
          .map((item) => ({
            id: item.id,
            name: item.name ?? item.id,
            description: item.manifest?.description ?? '',
            version: typeof item.version === 'number' ? item.version : undefined,
            status: item.status === 'failed' ? 'failed' : 'ok',
            error: typeof item.error === 'string' ? item.error : undefined,
          }));
        if (!cancelled) {
          useAppStore.getState().setSkills(summaries);
        }
        for (const item of list) {
          if (typeof item.id !== 'string') continue;
          const curVersion = typeof item.version === 'number' ? item.version : undefined;
          const curStatus: 'ok' | 'failed' = item.status === 'failed' ? 'failed' : 'ok';
          const prev = known.get(item.id);
          if (prev !== undefined) {
            const versionBumped =
              curVersion !== undefined && prev.version !== undefined && curVersion > prev.version;
            const statusChanged = prev.status !== curStatus;
            if (versionBumped || statusChanged) {
              handleEvent({
                type: 'skill-reloaded',
                skillId: item.id,
                skillName: item.name ?? item.id,
                status: curStatus,
                version: curStatus === 'failed' ? 'old-kept' : 'new',
                error: curStatus === 'failed' ? item.error : undefined,
              });
            }
          }
          known.set(item.id, { version: curVersion, status: curStatus });
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
      if (pollTimer !== null) clearInterval(pollTimer);
    };
    // Re-subscribe if the poll config changes (rare; included for completeness).
  }, [pollPath, pollIntervalMs, opts.disablePolling]);
}
