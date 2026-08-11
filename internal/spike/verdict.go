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

// Package spike — Verdict struct, the SINGLE SOURCE OF TRUTH for the
// compliance-review JSON schema produced by the ChatModelAgent pipeline.
//
// Per T-1-16 (Plan 01-04 threat register), this struct is the trust
// contract between the spike e2e pipeline (cmd/spiked/e2e.go) and the
// version-pinned integration contract test (integration/contract/
// contract_test.go). Drift between the two would silently break SC-5
// (contract enforcement) because the contract test would validate a
// different schema than the production pipeline emits.
//
// Mitigation: BOTH packages import `spike.Verdict` / `spike.ValidStatuses`
// / `spike.Validate` from this file. A field rename here triggers a
// COMPILE-TIME error in both packages — runtime drift is impossible by
// construction.
//
// The JSON shape matches spike 003 (`.claude/skills/spike-findings-ai-agent/
// sources/003-structured-output-temperature-zero/main.go` lines 55-95) —
// the proven schema for structured-output compliance verdicts on GLM-5.2
// with `response_format:{type:json_object}` + `thinking:{type:disabled}`.
package spike

import "fmt"

// Verdict is the JSON schema produced by a compliance-review ChatModelAgent.
// All four fields are required in the LLM's JSON output; Citations may be
// an empty array but must not be nil (Go decodes a missing JSON array as
// nil — Validate rejects that to force the LLM to emit an explicit []).
//
// JSON tags match spike 003 verbatim — any rename is a wire-contract break
// caught at compile time by the import graph (T-1-16).
type Verdict struct {
	Status         string   `json:"status"`          // one of ValidStatuses keys
	Confidence     float64  `json:"confidence"`      // [0.0, 1.0]
	Citations      []string `json:"citations"`       // clause IDs / fixture paths; may be empty
	ReviewRequired bool     `json:"review_required"` // true if Confidence < 0.9 OR info insufficient
}

// ValidStatuses is the enum gate for Verdict.Status. Exported so the
// integration contract test can assert the enum contents (a typo here
// would silently change semantics — `不合规` ≠ `不合规`<U+FE0F>; the
// contract test guards the exact rune sequence).
//
// The "unanswered" status is the graceful-refusal path: when the LLM
// lacks policy context, it returns status="unanswered" confidence=0
// citations=[] review_required=true. This was proven in spike 002/003
// (graceful-refusal finding).
var ValidStatuses = map[string]bool{
	"合规":      true,
	"不合规":     true,
	"需补充":     true,
	"unanswered": true,
}

// Validate checks that v has a known Status, a Confidence in [0, 1],
// and a non-nil Citations slice (empty is OK — nil is not). Returns
// (true, "") on pass, (false, reason) on fail.
//
// The reason string is suitable for inclusion in the e2e trace log
// (per D-12) — it is the audit trail for why a particular verdict was
// rejected. The reason NEVER contains the raw API key, only field values
// from v (which originate from the LLM response, not from env vars).
func Validate(v Verdict) (bool, string) {
	if !ValidStatuses[v.Status] {
		return false, fmt.Sprintf("invalid status: %q (must be one of 合规/不合规/需补充/unanswered)", v.Status)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return false, fmt.Sprintf("invalid confidence: %f (must be 0.0-1.0)", v.Confidence)
	}
	if v.Citations == nil {
		return false, "citations field missing (must be array, can be empty)"
	}
	return true, ""
}
