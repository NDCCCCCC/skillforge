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
// (cmd/spiked/sse_test.go, retained for the regression binary) for the
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
	ErrorText      string          `json:"errorText,omitempty"`      // error chunk payload (no API key ever)
	Data           json.RawMessage `json:"data,omitempty"`           // data-* custom payload (e.g. data-policy Verdict, data-decision DecisionPayload)
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
//   - Decision:     one of "retrieve" | "answer" | "refuse" | "forced_stop".
//   - ToolCallID:   links the decision to a specific tool invocation
//                   (empty when Eino does not expose the id at OnStart).
//   - Query:        the (possibly refined) query for a retrieve decision.
//   - Confidence:   model-reported confidence for an answer decision.
type DecisionPayload struct {
	Step       int     `json:"step"`                // 1-indexed hop number (1 or 2 for max_hops=2)
	Decision   string  `json:"decision"`            // "retrieve" | "answer" | "refuse" | "forced_stop"
	ToolCallID string  `json:"toolCallId,omitempty"`
	Query      string  `json:"query,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// ValidTypes enumerates the 17 kebab-case chunk types recognized by the
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
// Count: 17 types — text(3) + reasoning(3) + tool-input(3) +
// tool-output(1) + source(2) + data-policy(1) + data-decision(1) +
// finish(2) + error(1).
var ValidTypes = map[string]bool{
	"text-start":            true,
	"text-delta":            true,
	"text-end":              true,
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
	"finish-step":           true,
	"finish-message":        true,
	"error":                 true,
}

// IsValidType reports whether the chunk's Type field is a recognized
// kebab-case wire discriminator. Returns false for snake_case variants
// (e.g. "text_delta", "source_url") which the AI SDK v5 parser silently
// drops — EmitChunk uses this as the pre-write gate (T-1-11).
func (c UIMessageChunk) IsValidType() bool {
	return ValidTypes[c.Type]
}
