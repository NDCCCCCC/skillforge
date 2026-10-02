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

import "fmt"

// ValidStatuses is the platform-neutral enum gate for Verdict.Status.
// 260817-vsm: this is now skill-agnostic — every Skill (compliance /
// simple-qa / calculator / future) emits one of these four values. The
// compliance-specific verdict semantics (合规 / 不合规) live in the
// ComplianceStatus field on Verdict, not here.
//
// Semantics:
//
//	"ok"          — Skill produced an answer (factual answer in `Answer` for
//	                 simple-qa, OR a ComplianceStatus verdict for compliance).
//	"needs_input" — Information insufficient; UI prompts user for more.
//	                Replaces the old 需补充 (which was compliance-specific).
//	"refused"     — Graceful refusal: the Skill cannot / will not answer.
//	                Replaces the old "unanswered" (which was refusal-by-
//	                construction AND LLM-side opt-out combined). Pure LLM-
//	                side opt-out now uses compliance_status=不适用 for the
//	                compliance Skill; out-of-scope Skill routing uses the
//	                Refusal struct (not Verdict).
//	"error"       — Internal failure surfaced to the user. Distinct from
//	                "refused" so audit dashboards can distinguish user-
//	                facing refusal from system errors.
//
// The map is exported so the integration contract test can assert the
// enum contents — a typo here (e.g. a half-width vs full-width comma, a
// variation selector) would silently change semantics; the contract
// test guards the exact rune sequence.
var ValidStatuses = map[string]bool{
	"ok":          true,
	"needs_input": true,
	"refused":     true,
	"error":       true,
}

// Validate checks that v has a known Status, a Confidence in [0, 1],
// and a non-nil Citations slice (empty is OK — nil is not). Returns
// (true, "") on pass, (false, reason) on fail.
//
// The reason string is suitable for inclusion in the pipeline trace log
// (per D-12) — it is the audit trail for why a particular verdict was
// rejected. The reason NEVER contains the raw API key, only field values
// from v (which originate from the LLM response, not from env vars) —
// this is the T-1-13 information-disclosure mitigation carried forward
// from the spike.
//
// Validate does NOT verify that each citation's Span literally appears
// in any chunk — that is the grounding checker's job (grounding.go, D2-01).
// Validate only asserts the structural shape of the Verdict.
//
// 260817-vsm: the Compliance-Skill-specific verdict (合规 / 不合规 / 不适用)
// lives in ComplianceStatus, not Status. Validate does NOT enforce a
// ComplianceStatus enum — that is the Compliance Skill's responsibility
// (validated by the Compliance Skill's own tests, not the platform).
func Validate(v Verdict) (bool, string) {
	if !ValidStatuses[v.Status] {
		return false, fmt.Sprintf("invalid status: %q (must be one of ok/needs_input/refused/error)", v.Status)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return false, fmt.Sprintf("invalid confidence: %f (must be 0.0-1.0)", v.Confidence)
	}
	if v.Citations == nil {
		return false, "citations field missing (must be array, can be empty)"
	}
	return true, ""
}
