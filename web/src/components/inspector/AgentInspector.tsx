// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// AgentInspector — Plan 05-03 / SC-4 (AGNT-06) empty-state placeholder.
//
// The full Accordion-based inspector (per-hop groups + decision rows +
// tool-call expandables + retrieval chunk lists + reasoning collapse)
// ships in Plan 05-05. This plan mounts the empty + loading states so
// the 5-tab CenterPanel is fully wired and the inspector surface is
// visually discoverable from day one.
//
// Copywriting Contract (05-UI-SPEC §SC-4):
//   - Heading: `暂无 Agent 状态` (Display size)
//   - Body: `提出问题后，此处实时显示当前回合的工具调用、检索（含 chunk ID）与中间推理。`
//   - Loading: `等待 Agent 首个事件...` (shown when `streaming === true` and
//     no agent events have arrived yet)
//
// The inspector's eventual data source is `useChat.messages` filtered
// to {tool-*, data-decision, reasoning-*} parts (Claude's Discretion —
// no new instrumentation channel). For Phase 5/05-03 we don't read
// `messages` directly here; the empty/loading surface is mounted now,
// the rich tree follows in 05-05.
import { Activity } from 'lucide-react';

export interface AgentInspectorProps {
  /**
   * When true, renders the loading state `等待 Agent 首个事件...`
   * instead of the empty hero. Phase 5/05-05 will pass this from
   * useChat.status === 'streaming' && no events have arrived.
   */
  streaming?: boolean;
}

export default function AgentInspector({ streaming = false }: AgentInspectorProps) {
  if (streaming) {
    return (
      <div
        data-testid="agent-inspector-loading"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Activity className="h-6 w-6 text-primary animate-pulse" />
        </div>
        <div className="text-base text-text-muted">等待 Agent 首个事件...</div>
      </div>
    );
  }

  return (
    <div
      data-testid="agent-inspector-empty"
      className="flex flex-col items-center justify-center text-center min-h-[60vh]"
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
        <Activity className="h-6 w-6 text-primary" />
      </div>
      <div className="text-2xl font-semibold text-text">暂无 Agent 状态</div>
      <p className="mt-2 text-base text-text-muted max-w-sm">
        提出问题后，此处实时显示当前回合的工具调用、检索（含 chunk ID）与中间推理。
      </p>
    </div>
  );
}
