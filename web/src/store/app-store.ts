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

import { create } from 'zustand';
import type { Tier } from '../types/verdict';

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
    }
  | { kind: 'skill-md'; skillId: string };

export interface AppState {
  currentSkillId: string | null;
  centerView: CenterView;
  sidebarLeft: boolean;
  sidebarRight: boolean;
  preferences: { reasoningExpanded: boolean };
  leftWidth: number;
  rightWidth: number;
}

export interface AppActions {
  setCenterView(view: CenterView): void;
  setCurrentSkill(id: string | null): void;
  toggleSidebar(side: 'left' | 'right'): void;
  setReasoningExpanded(b: boolean): void;
  setPanelWidth(side: 'left' | 'right', width: number): void;
}

const DEFAULT_LEFT_WIDTH = 260;
const DEFAULT_RIGHT_WIDTH = 420;

export const useAppStore = create<AppState & AppActions>((set) => ({
  currentSkillId: null,
  centerView: { kind: 'empty' },
  sidebarLeft: true,
  sidebarRight: true,
  preferences: { reasoningExpanded: false },
  leftWidth: DEFAULT_LEFT_WIDTH,
  rightWidth: DEFAULT_RIGHT_WIDTH,

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
}));
