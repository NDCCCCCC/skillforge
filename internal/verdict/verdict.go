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
//
// 260817-vsm platform-decoupling evolution: Verdict is the platform-agnostic
// response envelope shared by all Skills (compliance / simple-qa / calculator
// / future). The platform-level Status is neutral ({ok, needs_input,
// refused, error}) — compliance-specific verdict semantics live in
// ComplianceStatus ({合规, 不合规, 不适用}). Factual queries (「X 是多少/谁/
// 何时/怎么办」) populate Answer with the direct answer; the conclusionText
// renderer leads with Answer when present, otherwise with ComplianceStatus.
package verdict

// Verdict is the platform-agnostic JSON schema for a Skill's response. It is
// produced by every Skill's LLM call (compliance / simple-qa / calculator /
// future Skills) and validated by Validate against the platform-neutral
// ValidStatuses enum.
//
// Field semantics:
//
//	Status         — platform-neutral: one of {ok, needs_input, refused, error}.
//	Confidence     — [0.0, 1.0]; semantics: "how confident is the LLM that the
//	                 citations grounding support the Answer / ComplianceStatus".
//	Citations      — D2-06 structured citations; may be empty (must NOT be nil
//	                 — Validate rejects nil to force the LLM to emit explicit []).
//	ReviewRequired — true when Confidence < 0.9 OR grounding downgrade applied.
//	Answer         — direct answer for factual queries (e.g. "由部门负责人审批").
//	                 Optional: empty for compliance-review queries where the
//	                 answer IS the ComplianceStatus. Rendered as conclusion
//	                 main body when non-empty (260817-vsm).
//	ComplianceStatus — Compliance-Skill-specific verdict: one of
//	                 {合规, 不合规, 不适用}. Optional: empty for non-compliance
//	                 Skills (simple-qa / calculator). The Compliance Skill's
//	                 system prompt instructs the LLM to populate this whenever
//	                 Status="ok" AND the query is a compliance review.
//
// JSON tags match spike 003 verbatim for Status / Confidence /
// ReviewRequired. Citations changes shape per D2-06. Answer and
// ComplianceStatus are 260817-vsm additions; both are omitempty so legacy
// producers (and Skills that don't use them) round-trip cleanly.
type Verdict struct {
	Status           string     `json:"status"`                     // platform-neutral (ValidStatuses)
	Confidence       float64    `json:"confidence"`                 // [0.0, 1.0]
	Citations        []Citation `json:"citations"`                  // D2-06: structured citations; may be empty
	ReviewRequired   bool       `json:"review_required"`            // true if Confidence < 0.9 OR grounding downgrade
	Answer           string     `json:"answer,omitempty"`           // 260817-vsm: factual-query direct answer
	ComplianceStatus string     `json:"compliance_status,omitempty"` // 260817-vsm: Compliance-Skill-specific verdict
}
