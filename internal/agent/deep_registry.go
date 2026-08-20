// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package agent — deep_registry.go is Phase 12 / Plan 12-02's
// DeepAgent Tool factory. The runtime/deep_agent.go factory
// (12-01) calls DeepAgentTools to obtain the per-DeepAgent Tool
// slice that gets passed to deep.Config.ToolsConfig.ToolsNodeConfig.
// Tools.
//
// Phase 12 / Plan 12-02 owns:
//
//   - write_todos (writeTodosAuditedTool — Plan 12-02 ships the
//     3-effect contract: audit INSERT + SSE chunk + session value)
//
// Phase 12 / Plan 12-03 will append:
//
//   - plan_approve (HITL-gated approval Tool — D11-C4
//     approval_id == tool_call_id PK UNIQUE constraint)
//
// Phase 12 / Plan 12-04 will append (or extend an existing Tool):
//
//   - retrieve (read-only knowledge retrieval — already lives in
//     internal/agent/tools/retrieve.go, the deep archetype reuses
//     it via BuildToolsForSkill in 12-01's runtime.DispatchAgent)
//
// Why a SEPARATE registry (vs extending BuildToolsForSkill)?
//
//   - BuildToolsForSkill lives in internal/agent/hitl_tools.go and
//     is the D11-B1 HITL whitelist recompute (intersection of
//     AllowedWriteTargets ∩ manifest.Writes). The deep archetype's
//     write_todos Tool is NOT a HITL Tool — it has no target, no
//     allowlist, no operator interrupt (D8-04 invariant). Forcing
//     it through BuildToolsForSkill would couple the write_todos
//     audit substrate to the HITL registry it has no relationship
//     with.
//   - The deep archetype's Tool list is built per-Runner (per the
//     same D11-B4 per-session Runner singleton invariant), but
//     BuildToolsForSkill is for the chat archetype's HITL Tools.
//     The deep archetype reuses the chat Tool whitelist for
//     retrieve + (future) plan_approve, then APPENDS write_todos
//     via this factory.
//
// CALL SITE (per DeepAgent construction in runtime.BuildDeepAgent):
//
//	tools, _ := agent.DeepAgentTools(cfg.Store, cfg.SessionID, emitter)
//	// append to the chat archetype's whitelist
//	for _, t := range tools { cfg.Tools = append(cfg.Tools, t) }
//	deep.New(ctx, &deep.Config{ ..., ToolsConfig: { ToolsNodeConfig: { Tools: cfg.Tools }}})
package agent

import (
	"github.com/cloudwego/eino/components/tool"

	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// DeepAgentTools returns the per-DeepAgent Tool slice. The slice is
// in the order the LLM should see the Tools in its schema:
//
//  1. write_todos (declared in the system prompt as the FIRST Tool
//     the LLM should call — emits a JSON todo plan BEFORE the first
//     real action; see runtime.deepArchetypePrefix).
//
//  2. (12-03) plan_approve — HITL-gated approval Tool. Lands in
//     12-03.
//
// 3. (12-04) retrieve_compliance_chunks — read-only retrieval Tool.
// The chat archetype's BuildToolsForSkill + BuildRetrieveTool
// already produce this; the deep factory appends them on top of
// write_todos via runtime.DispatchAgent.
//
// The Tool slice is passed straight to deep.Config.ToolsConfig.ToolsNodeConfig.Tools
// — the eino deep.New constructor wires them into the DeepAgent's
// tool-call loop.
//
// Required fields: store (audit substrate), runID (per-session
// identity), emitter (per-Run SSE seam). All three are mandatory
// because the DeepAgent archetype requires write_todos (the
// WithoutWriteTodos: true setting in 12-01 means the auto-injected
// Tool is OFF; this is the replacement).
//
// The 12-06 plan adds the Hertz-backed emitter implementation;
// until then, callers pass noopWriteTodosEmitter via BuildWriteTodosTool
// when no SSE is wired (unit-test posture).
func DeepAgentTools(store *sqlite.SQLiteStore, runID string, emitter WriteTodosEmitter) []tool.InvokableTool {
	writeTodos := BuildWriteTodosTool(store, runID, emitter)
	return []tool.InvokableTool{writeTodos}
}