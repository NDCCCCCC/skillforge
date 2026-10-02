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

package verdict

// Citation is the D2-04 minimal citation object: the smallest wire shape
// that satisfies ARAG-04 ("every cited claim = chunk ID + span"). Each
// field is required and non-empty for a grounded citation; the grounding
// checker (internal/verdict/grounding.go) verifies that Span literally
// appears in the chunk text identified by ChunkID after normalization.
//
// JSON tags are the wire contract pinned by integration/contract/
// contract_test.go TestContractStructuredOutputSchema — any rename is a
// wire-contract break caught at compile time by the import graph (T-1-16).
type Citation struct {
	// ChunkID is the retrieved chunk identifier (e.g. the Milvus INT64 PK
	// rendered as a string). Required.
	ChunkID string `json:"chunk_id"`

	// Span is a verbatim quote (D2-03) from the chunk that supports the
	// claim. The grounding checker normalizes both Span and chunk text
	// (collapse whitespace + 全角→半角 + trim per D2-05) then asserts
	// substring-contains. Required and non-empty for a grounded citation.
	Span string `json:"span"`

	// Tier is the knowledge tier the chunk belongs to (权威 / 历史 /
	// 个人 / 草稿). Carried in the citation so the wire consumer (Phase 4
	// UI citation panel) can badge sources without re-querying the retriever.
	// Required.
	Tier string `json:"tier"`
}

// Refusal is the structured refusal shape per ARAG-05. When the verdict
// pipeline cannot produce a grounded answer (0 chunks from tier-filtered
// retrieval — D-09; retrieval score below threshold — D2-15; or the query
// is out of Skill scope — D2-13 out_of_scope), it emits a Refusal instead
// of a Verdict. Emitted on the wire as the payload of a `data-policy`
// UIMessageChunk so the React consumer (Phase 4) can render a structured
// refusal card.
//
// The three fields exactly satisfy ARAG-05; score and effective_date are
// deferred (D2-04 minimal set).
type Refusal struct {
	// Reason is the refusal reason enum value (D2-13). Must be one of the
	// RefusalReason* constants below.
	Reason string `json:"reason"`

	// CanRephrase is true when the user could rephrase the query to make
	// progress (e.g. include a tier keyword). False when the request is
	// fundamentally out of scope (no rephrasing would help).
	CanRephrase bool `json:"can_user_rephrase"`

	// SuggestedAlternativeQuery is a rule-based suggestion (D2-14 amended):
	// Phase 2 emits a rephrased query that includes the tier keyword
	// (e.g. "权威制度" appended). Empty when CanRephrase is false.
	// LLM-generated suggestions are deferred to Phase 5 polish — the
	// refusal path precedes any LLM call (D-09), so LLM-gen is structurally
	// impossible here.
	SuggestedAlternativeQuery string `json:"suggested_alternative_query"`
}

// Refusal reason enum (D2-13). The pipeline emits exactly one of these in
// Refusal.Reason. Pinned by integration/contract/contract_test.go and by
// the unit tests in internal/verdict/.
const (
	// RefusalReasonNoAuthorizedSource: SC-2 structural refusal —
	// SearchWithTierFilter returned 0 chunks for the Skill's resolved
	// tier(s). Emitted BEFORE any LLM call (D-09 refusal-by-construction).
	RefusalReasonNoAuthorizedSource = "no_authorized_source"

	// RefusalReasonLowRetrievalScore: GEVL-01 — max retrieval cosine
	// similarity < 0.3 (D2-15 placeholder threshold; Phase 3 eval
	// calibrates). Emitted BEFORE any LLM call.
	RefusalReasonLowRetrievalScore = "low_retrieval_score"

	// RefusalReasonOutOfScope: D2-13 — the query is outside the Skill's
	// domain (e.g. contact-info request against the compliance Skill).
	// Detected by the loader's skill-scope check or by the LLM returning
	// status="unanswered".
	RefusalReasonOutOfScope = "out_of_scope"
)
