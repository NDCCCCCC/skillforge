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

import { create } from 'zustand';
import type { Tier } from '../types/verdict';
import type { SkillScore } from '../types/wire-types';

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
  // Phase 5 — SC-4 inspector + SC-5 graph browser
  | { kind: 'agent-inspector' }
  | { kind: 'knowledge-graph' };

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
 * the user refreshes the page. */
export interface SessionMeta {
  id: string;
  name: string;
  createdAt: number;
}

export interface AppState {
  currentSkillId: string | null;
  centerView: CenterView;
  sidebarLeft: boolean;
  sidebarRight: boolean;
  preferences: { reasoningExpanded: boolean };
  leftWidth: number;
  rightWidth: number;
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
  // Plan 05-08 (IN-04) — loud-failure signal. Flipped true by
  // useSkillReload after 3 consecutive non-OK /api/skills responses.
  // LeftSidebar renders a banner above the 技能 block when true.
  skillServiceOffline?: boolean;
}

export interface AppActions {
  setCenterView(view: CenterView): void;
  setCurrentSkill(id: string | null): void;
  toggleSidebar(side: 'left' | 'right'): void;
  setReasoningExpanded(b: boolean): void;
  setPanelWidth(side: 'left' | 'right', width: number): void;
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
  // Plan 05-08 (IN-04) — loud-failure flag toggle.
  setSkillServiceOffline(offline: boolean): void;
}

const DEFAULT_LEFT_WIDTH = 260;
const DEFAULT_RIGHT_WIDTH = 420;

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
  sidebarLeft: true,
  sidebarRight: true,
  preferences: { reasoningExpanded: false },
  leftWidth: DEFAULT_LEFT_WIDTH,
  rightWidth: DEFAULT_RIGHT_WIDTH,
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
  // Plan 05-08 (IN-04) — loud-failure flag. Default false; flipped
  // true by useSkillReload after 3 consecutive non-OK /api/skills
  // responses. Reset to false on the first OK response.
  skillServiceOffline: false,

  setCenterView: (view) => set({ centerView: view }),
  setCurrentSkill: (id) => set({ currentSkillId: id }),
  toggleSidebar: (side) =>
    set((state) => ({
      sidebarLeft: side === 'left' ? !state.sidebarLeft : state.sidebarLeft,
      sidebarRight: side === 'right' ? !state.sidebarRight : state.sidebarRight,
    })),
  setReasoningExpanded: (b) =>
    set((state) => ({ preferences: { ...state.preferences, reasoningExpanded: b } })),
  setPanelWidth: (side, width) =>
    set((state) => ({
      leftWidth: side === 'left' ? width : state.leftWidth,
      rightWidth: side === 'right' ? width : state.rightWidth,
    })),
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
  // Plan 05-08 (IN-04) — loud-failure flag toggle. LeftSidebar
  // observes this and renders a banner above the 技能 block.
  setSkillServiceOffline: (offline) => set({ skillServiceOffline: offline }),
}));
