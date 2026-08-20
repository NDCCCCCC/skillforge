// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 2 — Zustand v5 app store.
//
// STATE BOUNDARY LOCKED per CONTEXT.md §Claude's Discretion:
//   - useChat (AI SDK) OWNS the message stream
//   - Zustand OWNS app-level UI state (NOT messages)
//
// BANNED: do NOT add a `messages` field to this store. useChat owns
// the message stream. Adding messages here creates two sources of truth
// that drift (Pitfall 7 in RESEARCH.md).
//
// Plan 05-03 — Phase 5 extensions:
//   - CenterView gains `agent-inspector` (SC-4) + `knowledge-graph` (SC-5)
//     kinds so the 5-tab center panel wires through setCenterView.
//   - `wizardMode` field for SC-3 (wizard UI state — populated by 05-04).
//   - `skillReloadStatus` keyed by skillId for SC-2 reload toasts
//     (consumed by ReloadToast).
//   - `lastRouting` mirrors the latest routing decision's skillScores
//     so SkillPicker (used as the ambiguity picker) can read synchronously.
//
// Plan 05-07 (D5-IA-01/03/04) — UI-local session + live-skill surface:
//   - `SkillSummary` is the normalized view of an item in the live
//     GET /api/skills response. The description field is sourced from
//     item.manifest.description (the 7 schema fields are NESTED under
//     manifest per .continue-here.md BLOCKING CONSTRAINT — reading
//     item.description directly yields undefined).
//   - `skills: SkillSummary[]` + `setSkills(list)` make the live skill
//     surface read by ChatInput selector (D5-IA-01), LeftSidebar 技能
//     block (D5-IA-04), and SkillManagerMenu wizard-only entry.
//   - `SessionMeta` + `sessions[]` + `activeSessionId` + `createSession` /
//     `setActiveSession` / `deleteSession` drive the left-sidebar
//     会话 list (D5-IA-02/03) and the session-scoped useChat in App.tsx.
//     Sessions store METADATA ONLY (D14 / Pitfall 7) — useChat owns
//     the message stream per session id; messages are in-memory and
//     lost on refresh (D5-IA-03).
//
// Plan 05-08 (IN-04) — `skillServiceOffline: boolean` is flipped true by
// useSkillReload after 3 consecutive non-OK /api/skills responses (the
// loud-failure signal). LeftSidebar renders a banner above the 技能
// block when the flag is true. The flag is reset to false on the first
// OK response. Pure UI feedback — no secret data crosses the boundary.
//
// Plan 06-04 — Phase 6 eval dashboard slice. INVARIANT (D14): the
// eval slice holds route + reports + audit rows + filters ONLY.
// It MUST NEVER contain a `messages` field — useChat owns the chat
// message stream on the chat route, and adding messages here would
// create two sources of truth that drift (Pitfall 7 in RESEARCH.md).
// The `/eval` route has its own (non-SSE) transport (D-04/D12) so the
// chat SSE wire is untouched.
//
// Plan 11-06 — Phase 11 HITL approval-gate slice (`approval`).
// INVARIANT (D14): the slice lives on its own branch of the store,
// independent of the useChat stream — the pending / terminal /
// resumeBanner state below never touches the chat stream. The
// ApprovalCard overlay + the SSE transport dispatch read/write this
// branch exclusively; POST /api/session/:id/approval/:approvalId is
// the only mutation channel besides the SSE data-decision dispatch.
//
// Plan 12-06 Task 2 — Phase 12 plan-trace slice (`plan`). INVARIANT
// (D14-extension): the slice is INDEPENDENT from `messages` and from
// `approval` — it owns the live planning-trace state per runId. Keys
// are runId (session-scoped) so plans persist across page refresh
// (localStorage hydration below). PlanTracePart reads `planByRunId`
// for live render; the SSE transport dispatch (Task 4) writes here on
// data-plan-trace chunks + as a synthetic fallback on write_todos
// tool-call parts.

import { create } from 'zustand';
import type { Tier } from '../types/verdict';
import type { PlanTodo, SkillScore } from '../types/wire-types';
import type {
  AuditFilter,
  AuditRow,
  FrozenSetInfo,
  IndexRun,
} from '../types/eval';

// ----------------------------------------------------------------------------
// Plan 11-06 — HITL approval-gate types (HITL-01/02/07 + D11-B4 + D11-C4)
// ----------------------------------------------------------------------------

/** The 6 ApprovalCard visual states (11-UI-SPEC §ApprovalCard). */
export type ApprovalStatus =
  | 'pending'
  | 'approved'
  | 'denied'
  | 'expired'
  | 'auto-passed'
  | 'failed';

/** D11-D3 binary scope — no global option. */
export type ApprovalScope = 'session' | 'skill';

export type JsonObject = Record<string, unknown>;

/** Live interrupt card state. approvalId === tool_call_id (D11-C4
 * single identity across the card, the audit chain, and the POST
 * route). */
export interface PendingApproval {
  approvalId: string;
  /** Registered handler name (e.g. "memory_facts_write"). */
  toolName: string;
  /** Write target allowlist entry ("memory:facts" | "review_report"). */
  target: string;
  /** Snapshot taken at interrupt time (byte-identity anchor). */
  frozenArgs: JsonObject;
  /** Agent's reason; falls back to 未提供具体说明 at render. */
  reason: string;
  /** epoch ms. */
  createdAt: number;
  expiresAt: number | null;
  skillVersion: string;
  promptHash: string;
  sessionId: string;
  skillId: string;
  /** Locked — a pending row is always 'pending'. */
  status: 'pending';
}

/** Terminal card state (approved / denied / expired / auto-passed /
 * failed — one of the 5 terminal visual states). */
export interface TerminalApproval {
  approvalId: string;
  status: 'approved' | 'denied' | 'expired' | 'auto-passed' | 'failed';
  frozenArgs: JsonObject;
  /** denied: user reason; failed: failure message; else ''. */
  reason: string;
  createdAt: number;
  resolvedAt: number;
  /** true when an approve-and-remember rule matched (HITL-08). */
  auto_passed?: boolean;
}

/** One resolved entry — the live in-session feed the 11-07
 * AuditTimelineTab merges with its server-paged rows. Bounded to 200
 * entries (ring buffer). */
export interface HistoryApproval {
  approvalId: string;
  status: ApprovalStatus;
  target: string;
  toolName: string;
  frozenArgs: JsonObject;
  createdAt: number;
  resolvedAt: number;
  reason: string;
  skillVersion: string;
  promptHash: string;
}

/** Cross-refresh resume toast (3s auto-dismiss owned by the UI). */
export interface ResumeBanner {
  approvalId: string;
  status: 'pending' | 'auto-passed' | 'expired';
  createdAt: number;
}

/** TS mirror of the BE GET /api/session/:id/approval row
 * (internal/server/approval_api.go pendingApproval — snake_case JSON
 * tags are the wire ground truth). `created_at` arrives as RFC3339
 * (Go time.Time). `rule_id` is the 11-05 base64url-JSON PK (optional). */
export interface PendingApprovalRow {
  approval_id: string;
  tool_name: string;
  target: string;
  frozen_args: JsonObject;
  reason?: string;
  created_at: string;
  state: 'active' | 'expired' | 'auto_passed';
  skill_version: string;
  prompt_hash: string;
  rule_id?: string;
}

/** The approval branch of the store. D14: independent of the chat
 * stream — no chat content crosses into this branch. */
export interface ApprovalSlice {
  pending: PendingApproval | null;
  terminal: TerminalApproval | null;
  history: HistoryApproval[];
  resumeBanner: ResumeBanner | null;

  setPending: (approval: PendingApproval) => void;
  resolvePending: (approvalId: string, terminal: TerminalApproval) => void;
  clearPending: () => void;
  clearTerminal: () => void;
  appendHistory: (entry: HistoryApproval) => void;
  setResumeBanner: (banner: ResumeBanner | null) => void;
  dismissResumeBanner: () => void;
  /** Hydrate from GET /api/session/:id/approval (cross-refresh
   * resume). Accepts the BE snake_case row (or null); ctx carries the
   * caller-known sessionId / skillId the BE row omits. */
  hydrateFromGet: (
    row: PendingApprovalRow | null,
    ctx?: { sessionId?: string; skillId?: string },
  ) => void;
}

// ----------------------------------------------------------------------------
// Plan 12-06 Task 2 — planning-trace slice (D14 extension: NOT in messages)
// ----------------------------------------------------------------------------

/** The live planning-trace state for one DeepAgent run. PlanTracePart
 * reads this from the plan slice (the authoritative state — D14); the
 * useChat message stream is consulted ONLY for the synthetic write_todos
 * tool-call fallback (Pitfall 6 mitigation), not as the source of truth.
 */
export interface PlanState {
  /** Ordered list as received in the latest data-plan-trace chunk. */
  todos: PlanTodo[];
  /** Latest epoch millis from data-plan-trace (PlanTracePayload.ts). */
  generatedAt: number;
  /** Mirrors the DeepAgent run state — flips to 'complete' on finish /
   * 'forced_stop' on the forced_stop decision. */
  status: 'idle' | 'streaming' | 'complete' | 'forced_stop';
}

export interface PlanSlice {
  planByRunId: Record<string, PlanState>;

  /** Replace the todo snapshot for a runId. Called on every data-plan-trace
   * chunk. The trace is idempotent (each chunk carries the FULL post-write
   * snapshot), so we just replace in-place. */
  setPlan: (runId: string, todos: PlanTodo[], ts: number) => void;

  /** Transition a plan to a terminal state (complete | forced_stop). */
  markPlanStatus: (runId: string, status: PlanState['status']) => void;

  /** Drop one run's plan (called when a session/run is closed). */
  clearPlan: (runId: string) => void;
}

/** Discriminated union for the center panel's active view (D-02). */
export type CenterView =
  | { kind: 'empty' }
  | { kind: 'verdict'; verdictId: string }
  | {
      kind: 'chunk-source';
      chunkId: string;
      tier: Tier;
      version: string | null;
      effectiveDate: string | null;
      /**
       * Plan 05-05 / SC-8 (CMPL-08) — epoch seconds when the chunk
       * becomes superseded (filtered out of retriever results).
       * Null = perpetual. The FilteredOutNotice banner renders ABOVE
       * the body when the operator navigates to a superseded chunk
       * from the graph browser.
       */
      effectiveTo: number | null;
    }
  | { kind: 'skill-md'; skillId: string }
  // Doc viewer — full markdown source of one knowledge corpus doc.
  // Entry points: left-sidebar FILES tree click + knowledge-graph doc-node
  // click. Backed by GET /api/knowledge/doc?tier=&file=.
  | { kind: 'doc-view'; tier: Tier; file: string }
  // Phase 5 — SC-4 inspector + SC-5 graph browser
  | { kind: 'agent-inspector' }
  | { kind: 'knowledge-graph' }
  // Plan 11-07 — HITL-04 audit timeline tab (DetailPanel 6th tab
  // 审计轨迹, between 结论 and 引用原文 per UI-SPEC §Position 3).
  | { kind: 'audit-timeline' };

/** SC-2 reload toast payload. Keyed by skillId in `skillReloadStatus`. */
export interface SkillReloadEntry {
  status: 'ok' | 'failed';
  /** 'new' = new version loaded; 'old-kept' = validation failed, prior version kept (D5-08). */
  version: 'new' | 'old-kept';
  /** Validation error string when status === 'failed'. */
  error?: string;
  /** epoch ms when the event arrived (used for debouncing toast spam). */
  ts: number;
  /** Human-readable skill name for the toast body. */
  skillName: string;
}

/** SC-3 wizard state — populated by 05-04 wizard controller; the
 * `active` flag gates RightChat's wizard vs normal chat header. */
export interface WizardMode {
  active: boolean;
  step: number;
  totalSteps: number;
  skillName?: string;
}

export interface SkillSummary {
  id: string;
  name: string;
  /** Read from item.manifest.description per BLOCKING CONSTRAINT.
   * Falls back to '' when the live response lacks a manifest. */
  description: string;
  version?: number;
  status: 'ok' | 'failed';
  error?: string;
}

/** UI-local session metadata (D14 + D5-IA-03). The message stream is
 * owned by useChat, NOT this store — sessions are keyed by id into
 * useChat({id}) so each session retains its in-memory messages until
 * the user refreshes the page.
 *
 * Session-management phase adds two OPTIONAL fields (still metadata
 * only — the D14 no-messages invariant is untouched):
 *   - lastActiveAt: epoch ms of the newest turn (BE roster hydration).
 *     Absent on locally-created sessions until their first turn syncs;
 *     consumers group/fallback via `lastActiveAt ?? createdAt`.
 *   - pinned: user pin (drawer 置顶 group). Persisted as an id list in
 *     localStorage['sf.session.pins'] — pins must survive a roster
 *     refresh even though roster rows arrive unpinned from the BE. */
export interface SessionMeta {
  id: string;
  name: string;
  createdAt: number;
  lastActiveAt?: number;
  pinned?: boolean;
}

/** localStorage key holding the pinned session-id array (JSON).
 * localStorage access is guarded (SSR / privacy-mode throws are
 * treated as "no pins stored"). */
const PINS_STORAGE_KEY = 'sf.session.pins';

/** Read the persisted pin set. Returns a Set for O(1) roster-merge
 * lookups; an unreadable/corrupt payload degrades to an empty set. */
export function loadSessionPins(): Set<string> {
  try {
    const raw = globalThis.localStorage?.getItem(PINS_STORAGE_KEY);
    if (!raw) return new Set();
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((x): x is string => typeof x === 'string'));
  } catch {
    return new Set();
  }
}

/** Persist the pin id array. Best-effort — a throwing localStorage
 * (privacy mode) is silently ignored (pins live in memory for the
 * session). */
function saveSessionPins(ids: string[]): void {
  try {
    globalThis.localStorage?.setItem(PINS_STORAGE_KEY, JSON.stringify(ids));
  } catch {
    /* best-effort */
  }
}

// ----------------------------------------------------------------------------
// Plan 12-06 Task 2 — plan slice localStorage persistence (PLAN-02 SC-2
// cross-refresh). The `messages` field stays FORBIDDEN on this slice —
// only plan state persists, NOT the chat stream.
// ----------------------------------------------------------------------------

/** localStorage key holding the plan-trace snapshot (JSON). Per-Skill
 * max_steps cap (P12-L7) bounds the per-run todo count; we cap at 50
 * runs to dodge the 5-10MB localStorage quota (see 12-06-PLAN §Risk 2). */
const PLAN_STORAGE_KEY = 'skillforge-plan';
const PLAN_RETENTION_MAX_RUNS = 50;

/** Read the persisted plan slice. Returns a fresh empty map on missing /
 * malformed / privacy-mode payloads. Pure defensive read — the BE is
 * NEVER trusted to materialize this state. */
function loadPersistedPlan(): Record<string, PlanState> {
  try {
    const raw = globalThis.localStorage?.getItem(PLAN_STORAGE_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {};
    }
    // Cap to the most-recent N runs by generatedAt to stay under quota.
    const entries = Object.entries(parsed as Record<string, PlanState>)
      .filter(([, v]) => v && typeof v === 'object' && Array.isArray(v.todos))
      .sort((a, b) => (b[1].generatedAt ?? 0) - (a[1].generatedAt ?? 0))
      .slice(0, PLAN_RETENTION_MAX_RUNS);
    return Object.fromEntries(entries);
  } catch {
    return {};
  }
}

/** Persist the plan slice. Best-effort — a throwing localStorage
 * (privacy mode) is silently ignored (plan state survives in-memory). */
function savePersistedPlan(plan: Record<string, PlanState>): void {
  try {
    globalThis.localStorage?.setItem(PLAN_STORAGE_KEY, JSON.stringify(plan));
  } catch {
    /* best-effort */
  }
}

/** Plan 06-04 (D6-01) — admin-mode route toggle. The dashboard
 * replaces the chat three-panel when `route === 'eval'`. */
export type AppRoute = 'chat' | 'eval';

/** Plan 06-04 (D6-04) — `/eval` has a sub-route for the audit table. */
export type EvalSubroute = 'dashboard' | 'audit';

export interface AppState {
  currentSkillId: string | null;
  centerView: CenterView;
  // Open Design redesign (B2) — chat-first layout. The old three-panel
  // toggles (sidebarLeft/Right + left/rightWidth) are gone; the detail
  // side panel (former center panel) is the only resizable shell state.
  detailPanelOpen: boolean;
  detailPanelWidth: number;
  preferences: { reasoningExpanded: boolean };
  // Phase 5 (05-03) — wizard + reload + last-routing mirror
  wizardMode: WizardMode;
  skillReloadStatus: Record<string, SkillReloadEntry>;
  lastRouting?: { skillScores: SkillScore[]; ambiguous: boolean; selected?: string; ts: number };
  // Plan 05-04 — SC-3 wizard success toast. Carries the Skill name +
  // on-disk path so the success copy can quote both ("Skill 已创建：
  // {name}。已注册并热加载。"). Set by App.tsx's wizard-success listener
  // (the hook fires the event; the listener persists it for ReloadToast
  // and any other consumers to read).
  lastCreatedSkill?: { name: string; path: string; ts: number };
  // Plan 05-07 (D5-IA-01/03/04) — live skill surface + UI-local sessions.
  skills: SkillSummary[];
  sessions: SessionMeta[];
  activeSessionId: string;
  // Session-management phase — the LEFT session panel, mirroring the
  // right DetailPanel's inline-panel pattern: open = inline column
  // (≥1280px, chat compresses but stays usable) / fixed overlay
  // (<1280px, scrim dismiss). Store-owned so both entry points (TopBar
  // MessagesSquare button + SessionPicker foot) dispatch it.
  sessionPanelOpen: boolean;
  /** Inline panel width in px (clamped 220–360 by ChatLayout). */
  sessionPanelWidth: number;
  // Plan 05-08 (IN-04) — loud-failure signal. Flipped true by
  // useSkillReload after 3 consecutive non-OK /api/skills responses.
  // LeftSidebar renders a banner above the 技能 block when true.
  skillServiceOffline?: boolean;
  // Plan 06-04 (D6-01..04) — admin-mode eval dashboard slice.
  // INVARIANT (D14): NO `messages` field on this slice. The eval route
  // uses its own JSON transport; the chat SSE wire is untouched.
  route: AppRoute;
  evalSubroute: EvalSubroute;
  indexReports: IndexRun[];
  auditRows: AuditRow[];
  auditFilter: AuditFilter;
  frozenBaseline?: FrozenSetInfo;
  /** Loading + error mirror for the eval reports fetch (UI-only). */
  evalStatus: 'idle' | 'loading' | 'ready' | 'error';
  evalError?: string;
  // Plan 260815-cos — DraftStreamChip state. status='idle' means
  // the chip is hidden (no in-flight stream). MockChatTransport's
  // onDraftUpdate callback writes here as it parses SSE chunks.
  draftStatus: 'idle' | 'pending' | 'verified' | 'refused';
  draftText: string;
  // Plan 11-06 — HITL approval-gate slice (D14: own branch,
  // independent of the chat stream; SkillPicker's D11-B4 disabled
  // state reads approval.pending !== null in 11-07).
  approval: ApprovalSlice;
  // Plan 12-06 — plan-trace slice (D14 extension: independent of
  // `messages` AND of `approval`; runs are keyed by runId, persisted
  // across page refresh via localStorage hydration below).
  plan: PlanSlice;
}

export interface AppActions {
  setCenterView(view: CenterView): void;
  setCurrentSkill(id: string | null): void;
  setDetailPanelOpen(open: boolean): void;
  toggleDetailPanel(): void;
  setDetailPanelWidth(width: number): void;
  setReasoningExpanded(b: boolean): void;
  // Phase 5 (05-03) — wizard / reload / routing actions
  setWizardMode(mode: WizardMode): void;
  setSkillReloadStatus(skillId: string, entry: SkillReloadEntry): void;
  clearSkillReloadStatus(skillId: string): void;
  setLastRouting(routing: { skillScores: SkillScore[]; ambiguous: boolean; selected?: string; ts: number }): void;
  // Plan 05-04 — wizard success mirror
  setLastCreatedSkill(s: { name: string; path: string; ts: number }): void;
  clearLastCreatedSkill(): void;
  // Plan 05-07 — live skills + sessions
  setSkills(list: SkillSummary[]): void;
  createSession(): string;
  setActiveSession(id: string): void;
  deleteSession(id: string): void;
  // Session-management phase — roster hydration (BE GET /api/sessions)
  // + rename + pin. hydrateSessions REPLACES the local list when the
  // backend roster is non-empty (mount-time only; a later createSession
  // appends normally). renameSession is the optimistic FE write — the
  // caller fires PATCH separately and reverts on failure. Pin state is
  // mirrored to localStorage['sf.session.pins'] on every toggle.
  hydrateSessions(list: SessionMeta[]): void;
  renameSession(id: string, name: string): void;
  toggleSessionPin(id: string): void;
  /** Session panel open/close (store-owned so sibling entry points share it). */
  setSessionPanelOpen(open: boolean): void;
  setSessionPanelWidth(width: number): void;
  // Plan 05-08 (IN-04) — loud-failure flag toggle.
  setSkillServiceOffline(offline: boolean): void;
  // Plan 06-04 — eval dashboard actions (no messages setter).
  setRoute(route: AppRoute): void;
  setEvalSubroute(sub: EvalSubroute): void;
  setIndexReports(runs: IndexRun[]): void;
  setAuditRows(rows: AuditRow[]): void;
  setAuditFilter(filter: AuditFilter): void;
  setFrozenBaseline(f: FrozenSetInfo | undefined): void;
  setEvalError(msg: string | null): void;
  setEvalStatus(s: 'idle' | 'loading' | 'ready' | 'error'): void;
  // Plan 260815-cos — draft channel mirror (status + buffered text).
  // MockChatTransport's onDraftUpdate callback writes here; RightChat
  // subscribes to render the DraftStreamChip. Setting status='idle'
  // clears the chip entirely.
  setDraft(status: 'idle' | 'pending' | 'verified' | 'refused', text: string): void;
}

const DEFAULT_DETAIL_OPEN = false;
const DEFAULT_DETAIL_WIDTH = 380;

// Plan 05-07 — session id generation. Sessions are UI-local and
// in-memory (D5-IA-03), so a stable monotonic counter is sufficient
// (we do not need UUID uniqueness across reloads — the page reloads
// wipe all session state). `INITIAL_SESSION_ID` seeds the boot state
// so activeSessionId is never empty when the app mounts.
let sessionCounter = 1;
const INITIAL_SESSION_ID = `session-${sessionCounter++}`;

export const useAppStore = create<AppState & AppActions>((set, get) => ({
  currentSkillId: null,
  centerView: { kind: 'empty' },
  detailPanelOpen: DEFAULT_DETAIL_OPEN,
  detailPanelWidth: DEFAULT_DETAIL_WIDTH,
  preferences: { reasoningExpanded: false },
  // Phase 5 defaults
  wizardMode: { active: false, step: 0, totalSteps: 0 },
  skillReloadStatus: {},
  lastRouting: undefined,
  // Plan 05-07 defaults — one boot session so the right panel renders
  // an active useChat instance immediately (D5-IA-03 + Vercel AI SDK v5
  // per-id chatStore semantics).
  skills: [],
  sessions: [{ id: INITIAL_SESSION_ID, name: '会话 1', createdAt: Date.now() }],
  activeSessionId: INITIAL_SESSION_ID,
  sessionPanelOpen: false,
  sessionPanelWidth: 280,
  // Plan 05-08 (IN-04) — loud-failure flag. Default false; flipped
  // true by useSkillReload after 3 consecutive non-OK /api/skills
  // responses. Reset to false on the first OK response.
  skillServiceOffline: false,
  // Plan 06-04 (D6-01..04) — eval dashboard defaults. NO messages
  // field on this slice (D14 invariant). The chat route is the
  // default; eval is opt-in via LeftSidebar / popstate.
  route: 'chat',
  evalSubroute: 'dashboard',
  indexReports: [],
  auditRows: [],
  auditFilter: { limit: 100, offset: 0 },
  frozenBaseline: undefined,
  evalStatus: 'idle',
  evalError: undefined,
  // Plan 260815-cos — draft channel defaults (no chip rendered).
  draftStatus: 'idle',
  draftText: '',
  // Plan 11-06 — HITL approval-gate slice. All mutations funnel
  // through these actions; resolvePending is guarded against stale
  // SSE events (approvalId must match the live pending row).
  approval: {
    pending: null,
    terminal: null,
    history: [],
    resumeBanner: null,

    setPending: (approval) =>
      set((state) => ({
        approval: { ...state.approval, pending: approval, terminal: null },
      })),

    resolvePending: (approvalId, terminal) =>
      set((state) => {
        // Guard: only resolve when the live pending row matches —
        // a stale SSE event (out-of-order delivery) is a no-op.
        if (!state.approval.pending || state.approval.pending.approvalId !== approvalId) {
          return state;
        }
        return {
          approval: { ...state.approval, pending: null, terminal },
        };
      }),

    clearPending: () =>
      set((state) => ({ approval: { ...state.approval, pending: null } })),

    clearTerminal: () =>
      set((state) => ({ approval: { ...state.approval, terminal: null } })),

    appendHistory: (entry) =>
      set((state) => ({
        approval: {
          ...state.approval,
          // Bounded ring buffer — the 11-07 AuditTimelineTab owns the
          // server-paged feed; this is just the live in-session tail.
          history: [...state.approval.history, entry].slice(-200),
        },
      })),

    setResumeBanner: (banner) =>
      set((state) => ({ approval: { ...state.approval, resumeBanner: banner } })),

    dismissResumeBanner: () =>
      set((state) => ({ approval: { ...state.approval, resumeBanner: null } })),

    hydrateFromGet: (row, ctx) =>
      set((state) => {
        if (row === null) return state;
        if (row.state === 'expired') {
          return {
            approval: {
              ...state.approval,
              terminal: {
                approvalId: row.approval_id,
                status: 'expired',
                frozenArgs: row.frozen_args,
                reason: 'Skill 或 prompt 已变更，无法恢复',
                createdAt: Date.parse(row.created_at) || Date.now(),
                resolvedAt: Date.now(),
              },
              resumeBanner: {
                approvalId: row.approval_id,
                status: 'expired',
                createdAt: Date.now(),
              },
            },
          };
        }
        return {
          approval: {
            ...state.approval,
            pending: {
              approvalId: row.approval_id,
              toolName: row.tool_name,
              target: row.target,
              frozenArgs: row.frozen_args,
              reason: row.reason || '未提供具体说明',
              createdAt: Date.parse(row.created_at) || Date.now(),
              expiresAt: null,
              skillVersion: row.skill_version,
              promptHash: row.prompt_hash,
              sessionId: ctx?.sessionId ?? '',
              skillId: ctx?.skillId ?? '',
              status: 'pending',
            },
            resumeBanner: {
              approvalId: row.approval_id,
              status: 'pending',
              createdAt: Date.now(),
            },
          },
        };
      }),
  },
  // Plan 12-06 Task 2 — plan-trace slice. State is keyed by runId so
  // multiple in-flight runs / sessions don't collide. Hydrated from
  // localStorage at store creation (PLAN-02 SC-2 cross-refresh);
  // every mutation persists the slice via savePersistedPlan so a
  // refresh restores the last live plan.
  plan: {
    planByRunId: loadPersistedPlan(),

    setPlan: (runId, todos, ts) =>
      set((state) => {
        const next = {
          ...state.plan.planByRunId,
          [runId]: {
            todos,
            generatedAt: ts,
            status: 'streaming' as const,
          },
        };
        savePersistedPlan(next);
        return { plan: { ...state.plan, planByRunId: next } };
      }),

    markPlanStatus: (runId, status) =>
      set((state) => {
        const existing = state.plan.planByRunId[runId];
        if (!existing) return state;
        const next = {
          ...state.plan.planByRunId,
          [runId]: { ...existing, status },
        };
        savePersistedPlan(next);
        return { plan: { ...state.plan, planByRunId: next } };
      }),

    clearPlan: (runId) =>
      set((state) => {
        const { [runId]: _drop, ...rest } = state.plan.planByRunId;
        savePersistedPlan(rest);
        return { plan: { ...state.plan, planByRunId: rest } };
      }),
  },

  setCenterView: (view) => set({ centerView: view }),
  setCurrentSkill: (id) => set({ currentSkillId: id }),
  setDetailPanelOpen: (open) => set({ detailPanelOpen: open }),
  toggleDetailPanel: () => set((state) => ({ detailPanelOpen: !state.detailPanelOpen })),
  setDetailPanelWidth: (width) => set({ detailPanelWidth: width }),
  setReasoningExpanded: (b) =>
    set((state) => ({ preferences: { ...state.preferences, reasoningExpanded: b } })),
  // Phase 5 actions
  setWizardMode: (mode) => set({ wizardMode: mode }),
  setSkillReloadStatus: (skillId, entry) =>
    set((state) => ({
      skillReloadStatus: { ...state.skillReloadStatus, [skillId]: entry },
    })),
  clearSkillReloadStatus: (skillId) =>
    set((state) => {
      const next = { ...state.skillReloadStatus };
      delete next[skillId];
      return { skillReloadStatus: next };
    }),
  setLastRouting: (routing) => set({ lastRouting: routing }),
  setLastCreatedSkill: (s) => set({ lastCreatedSkill: s }),
  clearLastCreatedSkill: () => set({ lastCreatedSkill: undefined }),
  // Plan 05-07 actions
  setSkills: (list) => set({ skills: list }),
  createSession: () => {
    const id = `session-${sessionCounter++}`;
    const name = `会话 ${sessionCounter - 1}`;
    set((state) => ({
      sessions: [...state.sessions, { id, name, createdAt: Date.now() }],
      activeSessionId: id,
    }));
    return id;
  },
  setActiveSession: (id) => set({ activeSessionId: id }),
  deleteSession: (id) => {
    const state = get();
    const remaining = state.sessions.filter((s) => s.id !== id);
    // Guarantee activeSessionId is never empty: if we just deleted the
    // active session, fall back to the first remaining session (or
    // mint a fresh default session if none remain).
    let active = state.activeSessionId;
    if (active === id) {
      if (remaining.length > 0) {
        active = remaining[0].id;
      } else {
        const freshId = `session-${sessionCounter++}`;
        const freshName = `会话 ${sessionCounter - 1}`;
        remaining.push({ id: freshId, name: freshName, createdAt: Date.now() });
        active = freshId;
      }
    }
    set({ sessions: remaining, activeSessionId: active });
  },
  // Session-management actions — roster hydration / rename / pin.
  hydrateSessions: (list) => {
    if (list.length === 0) return; // empty/null roster → keep local state
    const pins = loadSessionPins();
    const hydrated = list.map((s) => ({ ...s, pinned: pins.has(s.id) || s.pinned }));
    set((state) => {
      // activeSessionId: prefer the newest BE session (roster is
      // newest-first) unless the current active is itself in the
      // roster (a mid-session re-hydrate keeps the user's context).
      const active = hydrated.some((s) => s.id === state.activeSessionId)
        ? state.activeSessionId
        : hydrated[0].id;
      return { sessions: hydrated, activeSessionId: active };
    });
  },
  renameSession: (id, name) =>
    set((state) => ({
      sessions: state.sessions.map((s) => (s.id === id ? { ...s, name } : s)),
    })),
  toggleSessionPin: (id) => {
    const state = get();
    const next = state.sessions.map((s) =>
      s.id === id ? { ...s, pinned: !s.pinned } : s,
    );
    saveSessionPins(next.filter((s) => s.pinned).map((s) => s.id));
    set({ sessions: next });
  },
  setSessionPanelOpen: (open) => set({ sessionPanelOpen: open }),
  setSessionPanelWidth: (width) => set({ sessionPanelWidth: width }),
  // Plan 05-08 (IN-04) — loud-failure flag toggle. LeftSidebar
  // observes this and renders a banner above the 技能 block.
  setSkillServiceOffline: (offline) => set({ skillServiceOffline: offline }),
  // Plan 06-04 — eval dashboard actions. These never touch the
  // chat useChat stream; the chat wire is owned by useChat + the
  // SSE transport (D14 + D-04/D12 preserved by isolation).
  setRoute: (route) => set({ route }),
  setEvalSubroute: (sub) => set({ evalSubroute: sub }),
  setIndexReports: (runs) => set({ indexReports: runs }),
  setAuditRows: (rows) => set({ auditRows: rows }),
  setAuditFilter: (filter) => set({ auditFilter: filter }),
  setFrozenBaseline: (f) => set({ frozenBaseline: f }),
  setEvalError: (msg) => set({ evalError: msg ?? undefined, evalStatus: msg ? 'error' : 'ready' }),
  setEvalStatus: (s) => set({ evalStatus: s }),
  // Plan 260815-cos — draft channel writer.
  setDraft: (status, text) => set({ draftStatus: status, draftText: text }),
}));
