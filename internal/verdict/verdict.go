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

// Package verdict holds the production Verdict struct, the SINGLE SOURCE OF
// TRUTH for the compliance-review JSON schema produced by the ChatModelAgent
// pipeline.
//
// Per T-1-16 (Plan 01-04 threat register, graduated in Plan 02-01), this
// struct is the trust contract between:
//   - the verdict pipeline (02-03 cmd/server handler, which produces Verdicts)
//   - the version-pinned integration contract test (integration/contract/
//     contract_test.go, which validates the schema)
//   - the grounding checker (internal/verdict/grounding.go, which mutates
//     Verdicts to enforce the D2-01 hallucination moat)
//
// Drift between these consumers would silently break SC-5 (contract
// enforcement) because the contract test would validate a different schema
// than the production pipeline emits. Mitigation: ALL three packages import
// `verdict.Verdict` / `verdict.ValidStatuses` / `verdict.Validate` from this
// file. A field rename here triggers a COMPILE-TIME error in every consumer
// — runtime drift is impossible by construction.
//
// The JSON shape graduated from spike 003
// (`.claude/skills/spike-findings-ai-agent/sources/003-structured-output-
// temperature-zero/main.go` lines 55-95) — the proven schema for
// structured-output compliance verdicts on GLM-5.2 with
// `response_format:{type:json_object}` + `thinking:{type:disabled}`.
//
// D2-06 evolution: Citations was `[]string` (clause IDs) in Phase 1; Phase 2
// upgrades it to `[]Citation{{ChunkID, Span, Tier}}` so each citation can be
// programmatically verified by the grounding checker (D2-01).
package verdict

// Verdict is the JSON schema produced by a compliance-review ChatModelAgent.
// All four fields are required in the LLM's JSON output; Citations may be an
// empty array but must not be nil (Go decodes a missing JSON array as nil —
// Validate rejects that to force the LLM to emit an explicit []).
//
// JSON tags match spike 003 verbatim for Status / Confidence /
// ReviewRequired. Citations changes shape per D2-06: each element is now a
// structured {chunk_id, span, tier} object instead of a bare string. This is
// a deliberate wire-contract evolution pinned by the integration contract
// test (TestContractStructuredOutputSchema).
type Verdict struct {
	Status         string     `json:"status"`          // one of ValidStatuses keys
	Confidence     float64    `json:"confidence"`      // [0.0, 1.0]
	Citations      []Citation `json:"citations"`       // D2-06: structured citations; may be empty
	ReviewRequired bool       `json:"review_required"` // true if Confidence < 0.9 OR grounding downgrade applied (D2-02)
}
