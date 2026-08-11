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

// ValidStatuses is the enum gate for Verdict.Status. Exported so the
// integration contract test can assert the enum contents (a typo here
// would silently change semantics — `不合规` ≠ `不合规`<U+FE0F>; the
// contract test guards the exact rune sequence).
//
// The "unanswered" status is the graceful-refusal path: when the LLM
// lacks policy context, it returns status="unanswered" confidence=0
// citations=[] review_required=true. This was proven in spike 002/003
// (graceful-refusal finding) and carries verbatim into the production
// verdict package.
var ValidStatuses = map[string]bool{
	"合规":        true,
	"不合规":       true,
	"需补充":       true,
	"unanswered": true,
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
