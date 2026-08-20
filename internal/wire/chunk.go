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

// Package wire holds the wire-contract primitives shared by the Hertz SSE
// emitter (internal/wire/sse.go), the verdict pipeline (02-03), and the
// integration contract test. The types here mirror the Vercel AI SDK v5
// UIMessageChunk discriminated union so that Hertz-emitted frames parse
// cleanly through AI SDK v5's DefaultChatTransport on the React side.
//
// Graduated verbatim from internal/spike/uimsg.go in Plan 02-01 (MOVE + new
// package, not a copy — anti-pattern #1 drift guard). The spike package is
// retired; this is now the single source of truth for the chunk envelope.
//
// See .planning/phases/01-integration-spike-trust-contract/01-RESEARCH.md
// §Pattern 2 (verified against hertz@v0.10.6/pkg/protocol/sse/writer.go)
// and §Pitfall 6 (kebab-case type discriminator) for the contract.
package wire

import "encoding/json"

// UIMessageChunk mirrors the Vercel AI SDK v5 discriminated union wire
// shape. The Type field is a kebab-case discriminator (text-delta,
// reasoning-delta, source-url, etc.) per RESEARCH.md §Pitfall 6 — the
// AI SDK v5 parser (process-ui-message-stream.ts) silently drops any
// frame whose Type is snake_case (e.g. text_delta) or not in its
// recognized set. Field names are camelCase per the same spec.
//
// Each category of typed part uses a different subset of fields:
//   - text-*:        ID, Delta
//   - reasoning-*:   ID, Delta
//   - tool-input-*:  ID, ToolCallID, ToolName, Input, InputTextDelta
//   - tool-output-*: ID, ToolCallID, Output
//   - source-url:    ID, SourceID, URL, Title
//   - source-doc:    ID, SourceID, Title (plus document fields)
//   - data-*:        Data (raw JSON for custom Skill payloads)
//   - error:         ErrorText
//   - finish-*:      (no payload fields; just the Type marker)
//
// Marshal output preserves struct field order; see TestUIMessageChunkShape
// (scripts/spiked/sse_test.go, retained for the regression binary) for the
// exact JSON asserted per category.
//
// Plan 03-01 added the data-decision discriminator (D3-03 / SC-1
// observability) — it carries an agent's per-hop DecisionPayload via
// the Data field below. The TS mirror lives at
// web/src/types/wire-types.ts and is gated by the D15 compile-time
// drift alarm AssertAssignable<SkillForgeChunk, UIMessageChunk>.
type UIMessageChunk struct {
	Type           string          `json:"type"`                     // REQUIRED, kebab-case enum
	ID             string          `json:"id,omitempty"`             // text/reasoning/tool/source lifecycle id
	Delta          string          `json:"delta,omitempty"`          // text-delta / reasoning-delta payload
	ToolCallID     string          `json:"toolCallId,omitempty"`     // links tool-input-* to tool-output-*
	ToolName       string          `json:"toolName,omitempty"`       // tool name (e.g. "search")
	Input          json.RawMessage `json:"input,omitempty"`          // tool-input-available parsed args
	InputTextDelta string          `json:"inputTextDelta,omitempty"` // tool-input-delta streaming fragment
	Output         json.RawMessage `json:"output,omitempty"`         // tool-output-available result
	SourceID       string          `json:"sourceId,omitempty"`       // source-url / source-document id
	URL            string          `json:"url,omitempty"`            // source-url href
	Title          string          `json:"title,omitempty"`          // source-url / source-document label
	MediaType      string          `json:"mediaType,omitempty"`      // source-document MIME type (AI SDK v5 REQUIRED: text/markdown for policy clauses)
	ErrorText      string          `json:"errorText,omitempty"`      // error chunk payload (no API key ever)
	Data           json.RawMessage `json:"data,omitempty"`           // data-* custom payload (e.g. data-policy Verdict, data-decision DecisionPayload)
}

// ConflictChunk is one 历史-tier annotation row carried inside a
// counter_evidence DecisionPayload. Plan 05-06 / SC-6 / D5-13.
//
// This struct is the load-bearing type for the D5-13 trust moat: it
// carries the metadata of a chunk retrieved from the 历史 tier as
// ANNOTATION ONLY — the verdict's chunksByID map is NEVER populated
// with these chunks (the trust boundary is enforced structurally by
// emitCounterEvidence's signature: it does not receive chunksByID).
//
// Field semantics:
//   - ChunkID:       numeric chunk id rendered as a decimal string
//     (matches retriever.Hit.ID → strconv.FormatInt).
//   - Tier:          always "历史" today (the counter-evidence pass
//     queries the 历史 tier only); kept on the struct
//     so a future cross-tier expansion (e.g. 草稿) does
//     not need a wire change.
//   - Source:        slash-normalized corpus path (e.g. "history/2023-policy.md").
//   - Snippet:       1-line text snippet for the annotation row. The FE
//     renders this verbatim as a text node (NEVER HTML).
//   - EffectiveFrom: epoch-seconds effective_from (0 = perpetual).
//   - EffectiveTo:   epoch-seconds effective_to (0 = perpetual).
type ConflictChunk struct {
	ChunkID       string `json:"chunkId"`
	Tier          string `json:"tier"`
	Source        string `json:"source"`
	Snippet       string `json:"snippet"`
	EffectiveFrom int64  `json:"effectiveFrom,omitempty"`
	EffectiveTo   int64  `json:"effectiveTo,omitempty"`
}

// DecisionPayload is the JSON shape carried by data-decision SSE chunks.
// Plan 03-03's agent package populates it via Eino callbacks
// (newDecisionCallback — see 03-RESEARCH.md §Code Examples lines 670-676)
// so the React UI can render the agent's per-hop autonomous decisions
// (retrieve / answer / refuse / forced_stop) — this is the ARAG-01 /
// SC-1 observability substrate.
//
// Field semantics:
//   - Step:         1-indexed hop number (1 or 2 for max_hops=2).
//   - Decision:     one of "retrieve" | "answer" | "refuse" |
//     "forced_stop" | "route" | "counter_evidence".
//     The latter two were added in Plan 05-01 (D5-04) for
//     the skill-router observability surface.
//   - ToolCallID:   links the decision to a specific tool invocation
//     (empty when Eino does not expose the id at OnStart).
//   - Query:        the (possibly refined) query for a retrieve decision.
//   - Confidence:   model-reported confidence for an answer decision.
//   - SkillScores:  populated when Decision == "route"; ranked list of
//     candidate Skills with their cosine-similarity scores
//     and the Selected flag (exactly one entry has
//     Selected=true unless Ambiguous is true).
//   - Selected:     the SkillID of the chosen Skill on a "route"
//     decision (empty for non-routing decisions).
//   - Ambiguous:    true when the top-2 score gap is below the router's
//     confidence threshold and the agent defers to a
//     follow-up query; UI surfaces this as a clarification
//     prompt.
//   - ConflictChunks: populated when Decision == "counter_evidence"
//     (Plan 05-06 / SC-6 / D5-13). The 历史-tier
//     annotation rows surfaced AFTER the verdict so the
//     operator can see contradicting evidence WITHOUT
//     the chunks entering the verdict's grounding map.
//     Empty slice (or nil) when the counter-evidence
//     pass found no conflicts — the FE renders the empty
//     state copy.
type DecisionPayload struct {
	Step           int             `json:"step"`     // 1-indexed hop number (1 or 2 for max_hops=2)
	Decision       string          `json:"decision"` // "retrieve" | "answer" | "refuse" | "forced_stop" | "route" | "counter_evidence"
	ToolCallID     string          `json:"toolCallId,omitempty"`
	Query          string          `json:"query,omitempty"`
	Confidence     float64         `json:"confidence,omitempty"`
	SkillScores    []SkillScore    `json:"skillScores,omitempty"`    // D5-04 — populated only on "route" decisions
	Selected       string          `json:"selected,omitempty"`       // D5-04 — SkillID chosen on "route"
	Ambiguous      bool            `json:"ambiguous,omitempty"`      // D5-04 — true when top-2 gap is below threshold
	ConflictChunks []ConflictChunk `json:"conflictChunks,omitempty"` // Plan 05-06 / D5-13 — populated only on "counter_evidence" decisions
}

// SkillScore is one ranked candidate Skill in a DecisionPayload's
// SkillScores slice. Used by the React SkillForgeInspector (Plan 05-04)
// to render the router's confidence breakdown.
//
// Field semantics:
//   - SkillID:   the directory name of the Skill (e.g. "compliance").
//   - SkillName: human-readable name from Manifest.Description's first
//     sentence (or the Skill directory name if absent).
//   - Score:     cosine-similarity score from the embedder against the
//     user's query (range [0.0, 1.0]; BGE-M3 default).
//   - Selected:  exactly one entry in the slice has Selected=true
//     unless the DecisionPayload.Ambiguous flag is true.
type SkillScore struct {
	SkillID   string  `json:"skillId"`
	SkillName string  `json:"skillName"`
	Score     float64 `json:"score"`
	Selected  bool    `json:"selected"`
}

// SubprogressStep is the JSON payload carried by data-subprogress SSE
// chunks (quick plan 260815-0xb — the FEUI-06 channel). The VerdictPipeline
// emits one per stage so the React UI can render streaming progress
// during the LLM wait; the FE mirror is the SubprogressStep interface at
// web/src/types/verdict.ts rendered by DataSubprogressPart.tsx.
//
// Field semantics (must stay field-aligned with the FE enum):
//   - Step:  one of "retrieve" | "found_candidates" | "rerank" |
//     "self_verify" | "generate". The pipeline emits 4 of these
//     (rerank is reserved for a future reranker pass).
//   - Tier:  the tier being searched (retrieve / found_candidates only).
//   - Count: the candidate chunk count (found_candidates only).
type SubprogressStep struct {
	Step  string `json:"step"`
	Tier  string `json:"tier,omitempty"`
	Count int    `json:"count,omitempty"`
}

// PlanTodo is one row of the DeepAgent planning trace. Phase 12 / Plan
// 12-02 — write_todos audit Tool substrate. Mirrors
// github.com/cloudwego/eino/adk/prebuilt/deep.TODO shape field-for-field
// but is SkillForge-owned (P12-L7): we do not import the upstream type
// because (a) the audit row stores SkillForge-owned JSON in the
// plan_todos table and (b) eino's deep.TODO struct is currently
// registered with schema.RegisterName ("_eino_adk_prebuilt_deep_todo")
// — pulling it across the wire boundary would couple SkillForge to the
// upstream JSON wire name. Status enum: "pending" | "in_progress" |
// "completed" (matches the eino upstream jsonschema tag enum). Add
// fields here AND on the FE mirror (web/src/types/wire-types.ts) under
// the D15 lockstep invariant when a future Phase needs them; never
// repurpose existing values.
type PlanTodo struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm,omitempty"`
	Status     string `json:"status"`
}

// PlanTracePayload is the JSON payload carried by data-plan-trace SSE
// chunks (Phase 12 / Plan 12-02 — PLAN-02 live todo stream substrate +
// PLAN-05 replay-query substrate). The internal/agent/deep_audit.go
// writeTodosAuditedTool emits one data-plan-trace chunk on every Tool
// call so the React UI can render the live planning trace as the
// DeepAgent iterates (PLAN-02 BE substrate). The TS mirror lives at
// web/src/types/wire-types.ts (12-06 ships the FE side) and is gated by
// the D15 compile-time drift alarm AssertAssignable<SkillForgeChunk,
// UIMessageChunk>.
//
// Field semantics:
//   - RunID: the per-session DeepAgent run identity; matches the
//     audit_row.session_id + the replay query (QueryPlanTodos) input.
//     Used by the FE to scope the rendered trace to one conversation.
//   - Todos: the full current todo snapshot — replaces the prior
//     snapshot in the FE state. The trace is idempotent: each chunk
//     carries the FULL post-write list, not a delta. AI SDK v5 buffers
//     data-* chunks until the message ends; the FE must mirror the
//     post-write snapshot in its own state on each chunk.
//   - Ts: epoch-seconds when write_todos was invoked. The FE renders
//     this next to the todo list as the trace timestamp.
type PlanTracePayload struct {
	RunID string     `json:"runId"`
	Todos []PlanTodo `json:"todos"`
	Ts    int64      `json:"ts"`
}

// Phase 10 (Plan 10-02) — history-aware pipeline data-decision
// extensions (D10-B4 / D10-C3). The pipeline emits one data-decision
// chunk per history-relevant event so the FE can render a
// transparency hint ("已理解为: XX 制度第二条") and a budget-chip
// indicator. The DecisionPayload struct is UNCHANGED — D15 invariant
// (zero new wire types); the Step=0 + Decision=<one of these 4>
// combination reuses the existing wire envelope.
//
// FE mirrors (web/) for these constants are registered in Plan 10-03;
// they are NOT carried over the wire as new UIMessageChunk variants —
// only the existing data-decision payload shape is used.
//
// The string values are stable wire contract identifiers — changing
// any of them breaks the FE parser silently. Add new events here
// (and to the FE mirror) when a future Phase needs them; never
// repurpose existing values.
const (
	// DecisionHistoryProjected fires when VerdictPipeline loads + sanitizes
	// prior session turns and projects them into the LLM messages prefix.
	// Step=0, Decision="history_projected", Query=projected turn count.
	DecisionHistoryProjected = "history_projected"

	// DecisionHistoryBudgetTruncated fires when TruncateToCharBudget
	// dropped at least one message (the char-budget window collapsed).
	// Step=0, Decision="history_budget_truncated", Query=dropped count.
	DecisionHistoryBudgetTruncated = "history_budget_truncated"

	// DecisionHistoryQueryRewritten fires when the query rewriter
	// produced a non-empty rewrite (D10-C1 / D10-C3 — transparency hint).
	// Step=0, Decision="history_query_rewritten", Query=rewritten text.
	DecisionHistoryQueryRewritten = "history_query_rewritten"

	// DecisionHistoryClassifierRefused fires when the query-only
	// refusal classifier returned Decision="refuse" — the verdict LLM
	// is NOT invoked (Step 7 short-circuit). Step=0,
	// Decision="history_classifier_refused", Query=refusal reason.
	DecisionHistoryClassifierRefused = "history_classifier_refused"
)

// ValidTypes enumerates the 20 kebab-case chunk types recognized by the
// AI SDK v5 UIMessageChunk parser (see ai-sdk.dev streaming-data docs
// + process-ui-message-stream.ts). EmitChunk rejects any Type not in
// this set BEFORE writing to the wire — this is the T-1-11 mitigation
// (reject unknown / snake_case types server-side so they cannot reach
// the client parser and get silently dropped).
//
// Per CONTEXT.md D-03/D-04, AI SDK v5 supports data-* custom types for
// Skill-specific payloads. We register `data-policy` here as the
// canonical SkillForge custom type carrying the compliance Verdict
// (distinct from `source-url` which carries the citation URL). This
// aligns the whitelist with TestUIMessageChunkShape's iteration set.
//
// Plan 03-01 added `data-decision` (D3-03 / SC-1 observability) — it
// carries the agent's autonomous per-hop decisions (retrieve / answer /
// refuse / forced_stop) emitted from Eino callbacks in Plan 03-03. The
// TS mirror at web/src/types/wire-types.ts MUST stay in lockstep per
// the D15 two-PR drift invariant.
//
// Plan 05-01 adds `data-redline` (D5-14) — it carries the agent's
// clause-level redline proposals (originalClause / proposedRewrite /
// rationale / optional clauseId) emitted by the compliance Skill when
// it rewrites a specific clause. The TS mirror MUST gain the matching
// { type: 'data-redline'; id?: string; data: RedlinePayload } variant
// or the D15 drift alarm (AssertAssignable) fails to compile.
//
// Quick plan 260815-0xb adds `data-subprogress` — the FEUI-06 channel.
// The VerdictPipeline emits one chunk per pipeline stage (retrieve /
// found_candidates / generate / self_verify) so the React UI renders
// streaming progress DURING the LLM wait instead of a long blank gap.
// The payload mirrors the FE SubprogressStep interface
// (web/src/types/verdict.ts) via wire.SubprogressStep below; the TS side
// (web/src/types/wire-types.ts) already declares this variant, so the
// D15 drift alarm needs no change.
//
// Count: 22 types — text(5: text-start/text-delta/text-end +
// text-delta-draft/text-delta-draft-end) + reasoning(3) +
// tool-input(3) + tool-output(1) + source(2) + data-policy(1) +
// data-decision(1) + data-redline(1) + data-subprogress(1) +
// finish(3: finish / finish-step / finish-message) + error(1).
//
// Quick plan 260815-cos adds `text-delta-draft` + `text-delta-draft-end`
// — the draft streaming channel that carries LLM tokens BEFORE
// grounding. The discriminator uses the `draft-1` ID namespace so
// draft frames cannot be confused with the `msg-1` text part (which
// AI SDK v5 routes into message.parts[].text). useChat does NOT
// consume draft-* frames; the FE transport (MockChatTransport) keeps
// them in a separate buffer so the chip's status state machine can
// render 🟡 待校验 → ✅ 已验证 on grounding pass.
//
// Phase 12 / Plan 12-02 adds `data-plan-trace` (P12-L6 — new
// discriminator, NOT an overload of data-decision). It carries the
// DeepAgent write_todos snapshot on every Tool call so the React UI
// can render the live planning trace (PLAN-02 BE substrate) + serve
// the 12-04 replay endpoint + 12-07 replay UI (PLAN-05 BE substrate).
// The TS mirror lives at web/src/types/wire-types.ts; 12-06 ships
// the matching SkillForgeChunk variant in lockstep (D15 paired-commit
// invariant). snake_case variant `data_plan_trace` MUST stay rejected
// (T-1-11 silent-drop regression guard, parallel to the data_decision
// rejection in TestDecisionChunkType).
var ValidTypes = map[string]bool{
	"text-start":            true,
	"text-delta":            true,
	"text-end":              true,
	"text-delta-draft":      true,
	"text-delta-draft-end":  true,
	"reasoning-start":       true,
	"reasoning-delta":       true,
	"reasoning-end":         true,
	"tool-input-start":      true,
	"tool-input-delta":      true,
	"tool-input-available":  true,
	"tool-output-available": true,
	"source-url":            true,
	"source-document":       true,
	"data-policy":           true,
	"data-decision":         true,
	"data-redline":          true,
	"data-subprogress":      true,
	"data-plan-trace":       true,
	// `finish` is the message-level terminator in AI SDK v5's
	// uiMessageChunkSchema (distinct from finish-step / finish-message).
	// VerdictPipeline emits it as the final chunk; the whitelist gate
	// rejected it until now (live WARN "unknown chunk type \"finish\"").
	"finish":         true,
	"finish-step":    true,
	"finish-message": true,
	"error":          true,
}

// IsValidType reports whether the chunk's Type field is a recognized
// kebab-case wire discriminator. Returns false for snake_case variants
// (e.g. "text_delta", "source_url") which the AI SDK v5 parser silently
// drops — EmitChunk uses this as the pre-write gate (T-1-11).
func (c UIMessageChunk) IsValidType() bool {
	return ValidTypes[c.Type]
}
