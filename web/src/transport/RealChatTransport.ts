// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 4 — extends the RealChatTransport SSE parser to
// dispatch `data-plan-trace` chunks to the plan slice (runId from the
// payload). Also routes synthetic write_todos tool-call parts as a
// Pitfall 6 fallback (authoritative state remains the plan slice).
//
// Plan 260815-j4y Task 2 (GREEN) — RealChatTransport.
//
// Wraps the AI SDK v7 DefaultChatTransport for the prod (env='prod')
// path. The wrapper exists for ONE reason: AI SDK v7's UIMessageStream
// parser aborts on unknown chunk types (throws UIMessageStreamError
// when it sees `text-delta-draft`), and the backend (Phase 2 Hertz SSE)
// emits text-delta-draft / text-delta-draft-end frames for the draft
// channel before grounding verifies the LLM answer. MockChatTransport
// already filters those frames in its fixture loop; this wrapper does
// the same for the real SSE stream so the parser never sees them.
//
// Per-frame dispatch:
//   - text-delta-draft          → accumulate delta, emit onDraftUpdate('pending'), do NOT enqueue
//   - text-delta-draft-end      → emit onDraftUpdate('pending'), do NOT enqueue (status holds)
//   - data-policy with .status  → emit onDraftUpdate('verified'), enqueue (Verdict arrives)
//   - data-policy with .reason  → emit onDraftUpdate('refused', ''), enqueue (Refusal arrives)
//   - other types               → enqueue verbatim
//
// Draft buffer is LOCAL to each sendMessages() stream start() closure —
// matches MockChatTransport semantics, so each turn starts fresh (the
// chip clears on refusal is the test responsibility, not the transport's).
//
// Re-exports MockChatTransport's DraftStatus / DraftUpdate /
// DraftUpdateHandler / MockChatTransportDraftOptions so callers don't
// have to import from two places.

import type { ChatTransport, UIMessage, UIMessageChunk } from 'ai';
import {
  type DraftStatus,
  type DraftUpdate,
  type DraftUpdateHandler,
  type MockChatTransportDraftOptions,
} from './MockChatTransport';
import { useAppStore } from '../store/app-store';
import type { JsonObject, TerminalApproval } from '../store/app-store';
import { ownerTokenHeaders } from '../lib/owner-token';
import { DecisionForcedStop, DecisionPlanApproved, type PlanTodo } from '../types/wire-types';

export type { DraftStatus, DraftUpdate, DraftUpdateHandler, MockChatTransportDraftOptions };

// ----------------------------------------------------------------------------
// Plan 11-06 Task 3 — HITL approval-gate dispatch (D-04 + D15 preserved)
// ----------------------------------------------------------------------------
//
// The data-decision chunk's `decision` STRING union gained 4 values in
// Phase 11 ({interrupted, tool-resumed, tool-denied, rule-matched}).
// Those 4 are intercepted here and dispatched to the store's approval
// branch — they are NEVER enqueued to useChat (the gate UI derives
// from the store, D14; no new chunk types are introduced, D15; the
// existing mock↔Default byte-identity is untouched because every
// pre-existing decision string still flows through unchanged, D-04).
//
// Field casing on the wire: the BE emits DecisionPayload with
// camelCase tags (toolCallId, query); the interrupted payload adds
// approval fields (approvalId / toolName / target / frozenArgs / …).
// Every read below is defensive — a malformed payload degrades to
// empty fields instead of crashing the stream.

function asStr(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asNum(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null;
}

function asObj(v: unknown): JsonObject {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
    ? (v as JsonObject)
    : {};
}

/** Dispatch one of the 4 Phase 11 decision strings to the store's
 * approval branch. Returns true when the chunk was handled (caller
 * skips the enqueue); false for every pre-existing decision string
 * (caller enqueues verbatim — D-04 byte-identity). */
function dispatchApprovalDecision(data: Record<string, unknown>): boolean {
  const decision = asStr(data.decision);
  const approval = useAppStore.getState().approval;

  switch (decision) {
    case 'interrupted': {
      // HITL-01 — the agent invoked a write tool; the runner parked on
      // the interrupt. Surface the pending card from the store.
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      if (!approvalId) return false; // malformed — drop to the stream
      approval.setPending({
        approvalId,
        toolName: asStr(data.toolName),
        target: asStr(data.target) || asStr(data.query),
        frozenArgs: asObj(data.frozenArgs),
        reason: asStr(data.reason) || '未提供具体说明',
        createdAt: asNum(data.createdAt) ?? Date.now(),
        expiresAt: asNum(data.expiresAt),
        skillVersion: asStr(data.skillVersion),
        promptHash: asStr(data.promptHash),
        sessionId: asStr(data.sessionId),
        skillId: asStr(data.skillId),
        // Eino resume address — the pending-card POST forwards it as
        // interrupt_id. Without it the BE falls back to the approvalID
        // which re-interrupts the Tool instead of resuming.
        interruptId: asStr(data.interruptId),
        status: 'pending',
      });
      return true;
    }

    case 'tool-resumed': {
      // HITL-02 — operator approved; the runner resumed the tool.
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      if (!approvalId) return false;
      // Capture the pending row BEFORE resolvePending clears it.
      const prev = approval.pending;
      const autoPassed = data.auto_passed === true;
      const terminal: TerminalApproval = {
        approvalId,
        status: autoPassed ? 'auto-passed' : 'approved',
        frozenArgs: Object.keys(asObj(data.frozenArgs)).length > 0
          ? asObj(data.frozenArgs)
          : (prev?.frozenArgs ?? {}),
        reason: asStr(data.reason),
        createdAt: prev?.createdAt ?? Date.now(),
        resolvedAt: Date.now(),
        auto_passed: autoPassed,
      };
      approval.resolvePending(approvalId, terminal);
      approval.appendHistory({
        approvalId,
        status: terminal.status,
        target: prev?.target ?? asStr(data.query),
        toolName: prev?.toolName ?? '',
        frozenArgs: terminal.frozenArgs,
        createdAt: terminal.createdAt,
        resolvedAt: terminal.resolvedAt,
        reason: terminal.reason,
        skillVersion: prev?.skillVersion ?? '',
        promptHash: prev?.promptHash ?? '',
      });
      return true;
    }

    case 'tool-denied': {
      // HITL-02 — operator denied; the runner force-stopped the tool.
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      if (!approvalId) return false;
      const prev = approval.pending;
      const terminal: TerminalApproval = {
        approvalId,
        status: 'denied',
        frozenArgs: prev?.frozenArgs ?? {},
        reason: asStr(data.reason),
        createdAt: prev?.createdAt ?? Date.now(),
        resolvedAt: Date.now(),
      };
      approval.resolvePending(approvalId, terminal);
      approval.appendHistory({
        approvalId,
        status: 'denied',
        target: prev?.target ?? asStr(data.query),
        toolName: prev?.toolName ?? '',
        frozenArgs: terminal.frozenArgs,
        createdAt: terminal.createdAt,
        resolvedAt: terminal.resolvedAt,
        reason: terminal.reason,
        skillVersion: prev?.skillVersion ?? '',
        promptHash: prev?.promptHash ?? '',
      });
      return true;
    }

    case 'rule-matched': {
      // HITL-08 — an approve-and-remember rule auto-passed. No card UI
      // (the user never saw a pending card); history-only record.
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      // 56-03 PANEL-03: point the rules panel at the row that just
      // auto-passed (the panel consumes this when the SessionDrawer is
      // open; otherwise the state parks until it opens).
      const ruleId = asStr(data.ruleId);
      if (ruleId) {
        useAppStore.getState().approval.setHighlightApprovalRuleId?.(ruleId);
      }
      approval.appendHistory({
        approvalId,
        status: 'auto-passed',
        target: asStr(data.target) || asStr(data.query),
        toolName: asStr(data.toolName),
        frozenArgs: asObj(data.frozenArgs),
        createdAt: Date.now(),
        resolvedAt: Date.now(),
        reason: '记忆规则命中',
        skillVersion: asStr(data.skillVersion),
        promptHash: asStr(data.promptHash),
      });
      return true;
    }

    // Plan 12-07 Task 3 (P12-L3 + 12-04 BE + 12-05 BE) — extend the
    // data-decision `decision` STRING enum with 3 new values:
    //   forced_stop        → update plan slice to 'forced_stop'
    //                        (12-05 BE: budget gate triggered)
    //   plan-approved      → approval slice markApproved +
    //                        plan slice markPlanStatus('complete')
    //                        (12-04 BE: plan_api planApproveHandler
    //                        emitted decision='plan-approved')
    //   plan-denied        → approval slice markDenied +
    //                        plan slice markPlanStatus('forced_stop')
    //                        (12-04 BE: deny path)
    // All three ride the EXISTING data-decision chunk — D15 lockstep
    // (no new wire types, only enum extension; D-04 transport-swap
    // byte-identity preserved because the pre-existing decision
    // strings still flow through unchanged).
    case DecisionForcedStop: {
      const runId = asStr(data.runId) || asStr(data.toolCallId);
      if (!runId) return false;
      useAppStore.getState().plan.markPlanStatus(runId, DecisionForcedStop);
      return true;
    }

    case DecisionPlanApproved: {
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      const runId = asStr(data.runId);
      if (!approvalId) return false;
      // approval slice — synthesize the terminal approval row so the
      // plan-approval UI overlay collapses into the 已批准 state.
      const prev = approval.pending;
      const terminal: TerminalApproval = {
        approvalId,
        status: 'approved',
        frozenArgs: prev?.frozenArgs ?? asObj(data.frozenArgs),
        reason: asStr(data.reason),
        createdAt: prev?.createdAt ?? Date.now(),
        resolvedAt: Date.now(),
      };
      approval.resolvePending(approvalId, terminal);
      approval.appendHistory({
        approvalId,
        status: 'approved',
        target: prev?.target ?? asStr(data.target) ?? 'plan_approve',
        toolName: prev?.toolName ?? 'plan_approve',
        frozenArgs: terminal.frozenArgs,
        createdAt: terminal.createdAt,
        resolvedAt: terminal.resolvedAt,
        reason: terminal.reason,
        skillVersion: prev?.skillVersion ?? '',
        promptHash: prev?.promptHash ?? '',
      });
      // plan slice — mark the run's plan complete.
      if (runId) {
        useAppStore.getState().plan.markPlanStatus(runId, 'complete');
      }
      return true;
    }

    case 'plan-denied': {
      const approvalId = asStr(data.approvalId) || asStr(data.toolCallId);
      const runId = asStr(data.runId);
      if (!approvalId) return false;
      const prev = approval.pending;
      const terminal: TerminalApproval = {
        approvalId,
        status: 'denied',
        frozenArgs: prev?.frozenArgs ?? asObj(data.frozenArgs),
        reason: asStr(data.reason),
        createdAt: prev?.createdAt ?? Date.now(),
        resolvedAt: Date.now(),
      };
      approval.resolvePending(approvalId, terminal);
      approval.appendHistory({
        approvalId,
        status: 'denied',
        target: prev?.target ?? asStr(data.target) ?? 'plan_approve',
        toolName: prev?.toolName ?? 'plan_approve',
        frozenArgs: terminal.frozenArgs,
        createdAt: terminal.createdAt,
        resolvedAt: terminal.resolvedAt,
        reason: terminal.reason,
        skillVersion: prev?.skillVersion ?? '',
        promptHash: prev?.promptHash ?? '',
      });
      if (runId) {
        useAppStore.getState().plan.markPlanStatus(runId, DecisionForcedStop);
      }
      return true;
    }

    // Plan 13-03 (P13-L7 / D16 overload) — FAN-04 SC-4 audit timeline
    // substrate. The 3 fanout_* decision strings ride the EXISTING
    // data-decision discriminator (no new wire chunk types — D16
    // overload pattern). They surface the sub-agent trace (dispatched
    // → completed | failed) for the PlanInspector timeline.
    //
    // 13-03 ships the dispatch seam; the visual timeline component
    // lands in 13-05 (the audit timeline reuses the existing
    // dispatchApprovalDecision branch's store-side wiring). For now,
    // the chunks are intercepted here and the fields are reserved
    // for the 13-05 timeline component that reads them off the
    // dispatch seam.
    case 'fanout_dispatched':
    case 'fanout_completed':
    case 'fanout_failed': {
      // 子 agent 决策（fanout_*）—— 字段已预留，本期不做时间线。
      //
      // 载荷契约已定稿：runPath / transferFrom / transferTo / reason
      // 由后端稳定下发，这里统一读出并保留，等时间线组件接上这段
      // dispatch seam 后直接消费即可。刻意不写 TODO —— 剩下的只是
      // UI，字段本身不会再变，写 TODO 只会变成噪音。
      const reservedFanout = {
        runPath: asStr(data.runPath),
        transferFrom: asStr(data.transferFrom),
        transferTo: asStr(data.transferTo),
        reason: asStr(data.reason),
      };
      void reservedFanout;
      return true;
    }

    default:
      return false;
  }
}


/**
 * 断线 / 流失败的用户可见提示。
 *
 * 之前 SSE 流的失败只会把 error 抛给 useChat（status 变成 'error'），
 * 界面上没有任何提示 —— 用户看到的是"消息发出去了，然后什么都没
 * 发生"，把断线误读成"模型还在想"。这里统一走 window 'sf-toast'
 * 事件（components/layout/SfToast.tsx 监听并渲染），把失败原因
 * 摊到用户面前。
 */
function notifyStreamFailure(message: string): void {
  try {
    window.dispatchEvent(
      new CustomEvent('sf-toast', {
        detail: { kind: 'error', message: `连接中断：${message}` },
      }),
    );
  } catch {
    // 非浏览器环境（单测）——没有视口可提示，静默即可
  }
}

/** 是否是"用户主动中止"造成的 AbortError —— 那不是失败，不提示。 */
function isAbortError(err: unknown): boolean {
  if (err instanceof Error && err.name === 'AbortError') return true;
  return typeof DOMException !== 'undefined' && err instanceof DOMException && err.name === 'AbortError';
}

/** Options accepted by RealChatTransport. */
export interface RealChatTransportOptions {
  /** Backend endpoint (defaults to /api/chat — set in createTransport). */
  api: string;
  /** Optional fetch override (tests inject a vi.fn() stub here). */
  fetch?: typeof globalThis.fetch;
  /** Extra request headers (merged into content-type: application/json). */
  headers?: Record<string, string>;
}

export class RealChatTransport implements ChatTransport<UIMessage> {
  private readonly opts: RealChatTransportOptions;
  private readonly onDraftUpdate: DraftUpdateHandler;

  constructor(opts: RealChatTransportOptions, draftOpts: MockChatTransportDraftOptions = {}) {
    this.opts = opts;
    this.onDraftUpdate = draftOpts.onDraftUpdate ?? (() => {});
  }

  async sendMessages(options: {
    trigger: 'submit-message' | 'regenerate-message';
    chatId: string;
    messageId: string | undefined;
    messages: UIMessage[];
    abortSignal: AbortSignal | undefined;
  } & Record<string, unknown>): Promise<ReadableStream<UIMessageChunk>> {
    const fetchFn = this.opts.fetch ?? globalThis.fetch;
    const api = this.opts.api;
    const extraHeaders = this.opts.headers;
    // Capture the handler in a local — inside the ReadableStream
    // `start()` callback, `this` is bound to the underlying source
    // object (NOT the RealChatTransport instance), so we cannot
    // reference `this.onDraftUpdate` from inside the stream loop.
    const onDraftUpdate = this.onDraftUpdate;

    // Issue the POST to the backend. The backend (Phase 2 Hertz SSE)
    // responds with `text/event-stream` frames encoded as JSON
    // UIMessageChunk payloads per the AI SDK v5 wire protocol.
    //
    // x-session-id (session-management phase): chat.go resolves the
    // session partition from this header OR mints a random id — without
    // it the FE's session-N id never reaches the store, so every turn
    // lands in a throwaway session and /api/session/:id/history +
    // /api/sessions never see it. options.chatId IS the active session
    // id (App.tsx creates the Chat with id: activeSessionId).
    //
    // X-Owner-Token (后端最小身份层): SSE 传输走的是裸 fetch 而不是
    // apiFetch，所以身份头必须在这里单独注入 —— 否则后端无法把这次
    // 请求归属到当前浏览器的 owner。
    //
    // 取消 (cancel): 持有一个内部 AbortController，把 AI SDK 传入的
    // abortSignal 转发到它，再由 ReadableStream 的 cancel() 回调真正
    // abort 掉底层 fetch —— 用户点「停止」时连接立刻断开，而不是
    // 等后端把剩余的流写完。
    const abortController = new AbortController();
    const externalSignal = options.abortSignal;
    if (externalSignal) {
      if (externalSignal.aborted) {
        abortController.abort();
      } else {
        externalSignal.addEventListener('abort', () => abortController.abort(), { once: true });
      }
    }

    let response: Response;
    try {
      response = await fetchFn(api, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          ...ownerTokenHeaders(),
          ...(options.chatId ? { 'x-session-id': options.chatId } : {}),
          ...extraHeaders,
        },
        body: JSON.stringify({
          messages: options.messages,
          id: options.chatId,
          trigger: options.trigger,
          // Phase 17 (DASH-01): merge caller-supplied body extensions —
          // App.handleSend passes { skill } so the backend can resolve the
          // selected Skill instead of falling back to defaultSkill.
          ...((options.body as Record<string, unknown> | undefined) ?? {}),
        }),
        signal: abortController.signal,
      });
    } catch (err) {
      // 网络层失败（连接被拒 / DNS / CORS）—— 用户必须看得见，
      // 否则表现为"消息石沉大海"。
      if (!isAbortError(err)) {
        notifyStreamFailure(err instanceof Error ? err.message : String(err));
      }
      throw err;
    }

    if (!response.ok || !response.body) {
      notifyStreamFailure(`${api} 返回 ${response.status}`);
      return new ReadableStream<UIMessageChunk>({
        start(controller) {
          controller.error(
            new Error(
              `RealChatTransport: ${api} returned ${response.status} (body=${response.body ? 'present' : 'missing'})`,
            ),
          );
        },
      });
    }

    // The stream is owned by this closure — draftBuffer / draftStatus
    // are LOCAL so each sendMessages() invocation starts fresh (matches
    // MockChatTransport semantics; the chip clears on refusal is the
    // UI's responsibility, not the transport's).
    return new ReadableStream<UIMessageChunk>({
      async start(controller) {
        let draftBuffer = '';
        let draftStatus: DraftStatus = 'pending';
        // Capture the chatId (== session id, == runId for v1) so the
        // synthetic fallback below can scope its plan slice write.
        const activeChatId = options.chatId ?? '';

        const emitDraft = (next: DraftStatus, text: string) => {
          draftStatus = next;
          draftBuffer = text;
          onDraftUpdate({ status: next, draftText: text });
        };

        try {
          const reader = response.body!.getReader();
          const decoder = new TextDecoder();
          let pendingText = '';

          // SSE framing: `data: <json>\n\n` per event. Each ReadableStream
          // chunk may carry a partial frame (the fetch body is NOT
          // pre-splitted), so we accumulate into pendingText and split
          // on `\n\n` boundaries. Multi-line `data:` (lines starting
          // with `data:`) are joined with `\n` per the SSE spec.
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            pendingText += decoder.decode(value, { stream: true });

            // Split on the blank-line frame terminator. Each frame may
            // have multiple `data:` lines; join them with `\n` per spec.
            let frameEnd: number;
            while ((frameEnd = pendingText.indexOf('\n\n')) !== -1) {
              const frame = pendingText.slice(0, frameEnd);
              pendingText = pendingText.slice(frameEnd + 2);

              const dataLines: string[] = [];
              for (const line of frame.split('\n')) {
                if (line.startsWith('data:')) {
                  dataLines.push(line.slice(5).trimStart());
                }
                // Ignore `event:`, `id:`, `retry:` etc — the AI SDK
                // wire format only uses `data:` for chunk payloads.
              }
              if (dataLines.length === 0) continue;
              const payload = dataLines.join('\n');

              let chunk: UIMessageChunk;
              try {
                chunk = JSON.parse(payload) as UIMessageChunk;
              } catch {
                // Malformed frame — skip silently (matches
                // DirectChatTransport behavior: don't crash the stream
                // on a single bad frame; the next frame will likely
                // resync).
                continue;
              }

              // SkillForge wire extensions (text-delta-draft /
              // text-delta-draft-end) are NOT part of the upstream
              // AI SDK v7 UIMessageChunk union — so the structural
              // comparison must happen against the un-narrowed wire
              // shape. Cast through `unknown` to a structural view
              // for the discriminator check.
              const wireChunk = chunk as unknown as { type: string; delta?: string };

              // Draft channel — keep buffer up to date but never enqueue
              // the draft frame to useChat (which would route it into
              // message.parts[].text via the text-delta parser).
              if (wireChunk.type === 'text-delta-draft') {
                const delta = wireChunk.delta ?? '';
                emitDraft('pending', draftBuffer + delta);
                continue;
              }
              if (wireChunk.type === 'text-delta-draft-end') {
                // Status holds 'pending' until data-policy decides.
                emitDraft(draftStatus, draftBuffer);
                continue;
              }

              // data-policy is the trust-bearing transition: it carries
              // either a Verdict (verified → chip turns ✅ 已验证) or a
              // Refusal (refused → chip unmounts, RefusalCard takes over).
              if (wireChunk.type === 'data-policy') {
                const data = (chunk as { data?: { status?: string; reason?: string } }).data;
                if (data && typeof data === 'object') {
                  if (typeof data.status === 'string') {
                    emitDraft('verified', draftBuffer);
                  } else if (typeof data.reason === 'string') {
                    emitDraft('refused', '');
                  }
                }
              }

              // Plan 11-06 — HITL approval-gate decisions. Only the 4
              // Phase 11 decision strings are intercepted (dispatched
              // to the store, never enqueued — the gate UI derives from
              // the store, D14). Every pre-existing decision string
              // (route / retrieve / counter_evidence / history_* / …)
              // falls through to the verbatim enqueue below (D-04).
              if (wireChunk.type === 'data-decision') {
                const data = (chunk as { data?: Record<string, unknown> }).data;
                if (data && typeof data === 'object') {
                  if (dispatchApprovalDecision(data)) {
                    continue;
                  }
                }
              }

              // Plan 12-06 — live planning-trace dispatch (PLAN-02
              // SC-2). The data-plan-trace chunk carries the FULL
              // post-write todo snapshot for one runId (12-02 BE side:
              // writeTodosAuditedTool emits this). Route to the plan
              // slice via setPlan; the synthetic fallback path below
              // handles the write_todos tool-call parts if the AI SDK
              // v5 parser buffers data-* chunks until message end
              // (Pitfall 6). Authoritative state stays in the plan
              // slice (D14) — the chunk is NEVER enqueued to useChat.
              if (wireChunk.type === 'data-plan-trace') {
                const data = (chunk as { data?: { runId?: unknown; todos?: unknown; ts?: unknown } }).data;
                if (data && typeof data === 'object') {
                  const runId = typeof data.runId === 'string' ? data.runId : '';
                  const todos = Array.isArray(data.todos) ? (data.todos as PlanTodo[]) : [];
                  const ts = typeof data.ts === 'number' ? data.ts : Date.now();
                  if (runId && todos.length > 0) {
                    const planState = useAppStore.getState().plan;
                    planState.setPlan(runId, todos, ts);
                    continue;
                  }
                }
              }

              // Plan 260815-k8d — text-end is the terminal signal for the
              // conclusion text stream. The draft channel's own
              // text-delta-draft-end was intercepted above (continue),
              // so this branch only fires for the FINAL answer text-end.
              // Emit ('idle', '') so RightChat unmounts the
              // DraftStreamChip — without this the chip stays pinned at
              // "✅ 已验证" forever after every successful verdict.
              if (wireChunk.type === 'text-end') {
                emitDraft('idle', '');
              }

              // Plan 12-06 — synthetic write_todos tool-call fallback
              // (Pitfall 6). When AI SDK v5 buffers data-* chunks, the
              // authoritative path silently delays; the AI SDK's
              // tool-call lifecycle surface (tool-input-available +
              // tool-output-available) IS rendered live. We mirror the
              // write_todos output to the plan slice as a fallback so
              // PlanTracePart has fresh data even if the data-plan-
              // trace chunk is buffered. The chunk is NOT consumed —
              // it still flows to useChat as the synthetic path
              // requires (useChat.messages.parts is consulted).
              if (wireChunk.type === 'tool-output-available') {
                const tOut = chunk as { toolCallId?: string; output?: unknown };
                if (tOut.output && typeof tOut.output === 'object') {
                  const out = tOut.output as { toolName?: unknown; output?: unknown };
                  // The tool-call part's input/output are nested under
                  // generic `output` of the dispatcher; in our wire,
                  // the runner emits the write_todos snapshot in the
                  // `output` field. Best-effort extract.
                  const payload = out.output as { todos?: unknown } | undefined;
                  if (
                    out &&
                    typeof out.toolName === 'string' &&
                    out.toolName === 'write_todos' &&
                    payload &&
                    Array.isArray(payload.todos)
                  ) {
                    const todos = payload.todos as PlanTodo[];
                    if (todos.length > 0) {
                      const runId = `${activeChatId || 'unknown'}-synthetic`;
                      useAppStore
                        .getState()
                        .plan.setPlan(runId, todos, Date.now());
                    }
                  }
                }
              }

              controller.enqueue(chunk);
            }
          }

          controller.close();
        } catch (err) {
          if (isAbortError(err)) {
            // 用户主动中止（点「停止」）—— 不是失败，不弹提示；
            // 正常收尾，让 useChat 把 status 复位为 ready。
            try {
              controller.close();
            } catch {
              // 流已被消费方关闭 —— 忽略
            }
            return;
          }
          // 真实断线：既要让 useChat 进入 error 状态，也要把原因
          // 摊给用户（静默失败会被误读成"模型还在想"）。
          controller.error(err);
          notifyStreamFailure(err instanceof Error ? err.message : String(err));
        }
      },
      cancel(reason) {
        // 消费者取消（useChat 的 stop()）→ 真正 abort 底层 fetch，
        // SSE 连接立刻断开；后端随之看到客户端断开并停止推理。
        // 旧实现留空并声称"reader 释放后 body 会自动关闭"—— 那是
        // 错的：body 只在 reader 释放后才结束，而 cancel() 正是
        // 释放之前的那一环。
        abortController.abort(reason);
      },
    });
  }

  async reconnectToStream(_options: {
    chatId: string;
    abortSignal?: AbortSignal;
  } & Record<string, unknown>): Promise<ReadableStream<UIMessageChunk> | null> {
    // 后端没有持久化的流句柄，断线后无法把上一段 SSE 续上（v1
    // 语义与 MockChatTransport / DirectChatTransport 一致）：返回
    // null 让 AI SDK 把这一轮标记为失败。断线的可见提示由
    // notifyStreamFailure 负责，不在这里静默吞掉。
    return null;
  }
}