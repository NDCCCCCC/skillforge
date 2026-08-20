// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 / HITL-03 — the cross-refresh resume hook. On App mount +
// activeSessionId change, fires the parallel recovery fetch:
//
//   Promise.all([
//     GET /api/session/:id/approval        (pending row | empty | error)
//     GET /api/session/:id/approval-rules  (panel cache warm)
//     GET /api/session/:id/history         (chat-restore warm)
//   ])
//
// Then exactly ONE branch fires (mutually exclusive):
//   - error (undefined)  → clearPending; NO banner (fail-closed — the
//     ApprovalCard error state surfaces the failure, 11-06 owned)
//   - empty (null)       → clearPending; NO banner
//   - state=active       → hydrateFromGet → approval.pending set +
//     resumeBanner{status:'pending'} (ApprovalCard renders, 11-06)
//   - state=expired      → hydrateFromGet → terminal{expired} +
//     resumeBanner{status:'expired'} (D11-C2 Skill drift)
//   - state=auto_passed  → terminal{auto-passed} + resumeBanner
//     {status:'auto-passed'} (HITL-08 rule matched — no pending card)
//
// The banner auto-dismisses after 3s (the underlying terminal/pending
// state persists — the ApprovalCard visual is independent of the
// banner). NO polling (UI-SPEC §"No polling" invariant) — one shot per
// mount + session change.
import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';
import { fetchApprovalRules, fetchPendingApproval } from '../lib/sessions';
import { fetchHistory } from '../lib/chat-restore';

/** Banner auto-dismiss window (UI-SPEC §Cross-refresh resume UI). */
export const RESUME_BANNER_MS = 3000;

/** HITL-03 banner copy keyed by banner status (the store's ResumeBanner
 * carries the status; the copy is a pure derivation — UI-SPEC §Copy). */
export const RESUME_BANNER_TEXT: Record<'pending' | 'auto-passed' | 'expired', string> = {
  pending: '恢复了一次中断的请求',
  'auto-passed': '恢复了一次中断的请求（记忆规则已通过）',
  expired: '恢复了一次中断的请求，但 Skill 已变更，审批已过期',
};

export function useResumeBanner(activeSessionId: string): void {
  useEffect(() => {
    let cancelled = false;
    let bannerTimer: ReturnType<typeof setTimeout> | null = null;

    void (async () => {
      const skillId = useAppStore.getState().currentSkillId;
      const [pending] = await Promise.all([
        fetchPendingApproval(activeSessionId),
        // The rules list + history are warmed in parallel; the
        // ApprovalRulesPanel owns its own fetch on mount, and App's
        // history-restore effect owns the chat stream — these two
        // calls are cache-warm companions of the resume read.
        fetchApprovalRules(activeSessionId, skillId),
        fetchHistory(activeSessionId),
      ]);
      if (cancelled) return;

      const approval = useAppStore.getState().approval;
      if (pending === undefined || pending === null) {
        // Fail-closed (error) + no-pending both collapse to "no
        // pending card"; neither raises a resume banner.
        approval.clearPending();
        return;
      }

      if (pending.state === 'auto_passed') {
        // HITL-08 — a remember-rule matched while the tab was away.
        // 11-06's hydrateFromGet covers active + expired; the
        // auto_passed branch terminalizes directly (NO pending card —
        // the operator never saw an interrupt for this row).
        useAppStore.setState((s) => ({
          approval: {
            ...s.approval,
            pending: null,
            terminal: {
              approvalId: pending.approval_id,
              status: 'auto-passed',
              frozenArgs: pending.frozen_args,
              reason: '记忆规则命中',
              createdAt: Date.parse(pending.created_at) || Date.now(),
              resolvedAt: Date.now(),
              auto_passed: true,
            },
            resumeBanner: {
              approvalId: pending.approval_id,
              status: 'auto-passed',
              createdAt: Date.now(),
            },
          },
        }));
      } else {
        // state=active → pending card + banner; state=expired →
        // terminal expired + banner (11-06's hydrateFromGet branches).
        approval.hydrateFromGet(pending, { sessionId: activeSessionId, skillId: skillId ?? undefined });
      }

      bannerTimer = setTimeout(() => {
        if (!cancelled) {
          useAppStore.getState().approval.dismissResumeBanner();
        }
      }, RESUME_BANNER_MS);
    })();

    return () => {
      cancelled = true;
      if (bannerTimer) clearTimeout(bannerTimer);
    };
  }, [activeSessionId]);
}
